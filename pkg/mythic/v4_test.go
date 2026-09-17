package mythic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestAuthHeadersBearerOnly(t *testing.T) {
	headers := AuthHeaders("mtk_example", "")
	if headers["Authorization"] != "Bearer mtk_example" {
		t.Fatalf("api token header = %v", headers)
	}
	if _, ok := headers["apitoken"]; ok {
		t.Fatal("apitoken header must not be set")
	}

	headers = AuthHeaders("mtk_example", "jwt-access")
	if headers["Authorization"] != "Bearer jwt-access" {
		t.Fatalf("access token should win, got %v", headers)
	}
}

func TestWebhookEndpointStripsV14Prefix(t *testing.T) {
	cases := map[string]string{
		"api/v1.4/create_task_webhook": "create_task_webhook",
		"/api/v1.4/tag_create_webhook": "tag_create_webhook",
		"create_task_webhook":          "create_task_webhook",
		"/task_upload_file_webhook":    "task_upload_file_webhook",
	}
	for in, want := range cases {
		if got := WebhookEndpoint(in); got != want {
			t.Errorf("WebhookEndpoint(%q)=%q want %q", in, got, want)
		}
	}
}

func TestDirectDownloadURL(t *testing.T) {
	client, err := NewClient(&Config{
		ServerURL: "https://mythic.example:7443",
		APIToken:  "mtk_test",
		SSL:       true,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	got := client.directDownloadURL("abc-uuid")
	want := "https://mythic.example:7443/direct/download/abc-uuid"
	if got != want {
		t.Fatalf("directDownloadURL = %q want %q", got, want)
	}
}

func TestLiveGetMeAndDownload(t *testing.T) {
	if os.Getenv("MYTHIC_LIVE") != "1" {
		t.Skip("set MYTHIC_LIVE=1 with MYTHIC_URL and MYTHIC_APITOKEN")
	}
	url := os.Getenv("MYTHIC_URL")
	token := os.Getenv("MYTHIC_APITOKEN")
	if token == "" {
		token = os.Getenv("MYTHIC_API_TOKEN")
	}
	if url == "" || token == "" {
		t.Fatal("MYTHIC_URL and MYTHIC_APITOKEN required")
	}

	client, err := NewClient(&Config{
		ServerURL:     url,
		APIToken:      token,
		SSL:           true,
		SkipTLSVerify: true,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	op, err := client.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if op.ID == 0 || op.Username == "" {
		t.Fatalf("empty operator: %+v", op)
	}

	fileID, err := client.UploadFile(context.Background(), "whoami-fix.txt", []byte("whoami-fix"))
	if err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	data, err := client.DownloadFile(context.Background(), fileID)
	if err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	if string(data) != "whoami-fix" {
		t.Fatalf("downloaded %q", data)
	}

	if err := client.UpdateCurrentOperationForUser(context.Background(), 1); err != nil {
		t.Fatalf("UpdateCurrentOperationForUser: %v", err)
	}
}

func TestGetMeUsesWhoamiNotRestMe(t *testing.T) {
	var sawMe, sawWhoami bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/me" {
			sawMe = true
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, "/graphql") {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "whoami") {
			sawWhoami = true
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"whoami": map[string]any{
					"status":               "success",
					"error":                nil,
					"user_id":              1,
					"username":             "mythic_admin",
					"admin":                true,
					"active":               true,
					"current_operation_id": 1,
					"current_operation":    "Default",
				},
			},
		})
	}))
	defer srv.Close()

	client, err := NewClient(&Config{
		ServerURL: srv.URL,
		APIToken:  "mtk_test",
		SSL:       false,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	op, err := client.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if sawMe {
		t.Fatal("GetMe must not call GET /me")
	}
	if !sawWhoami {
		t.Fatal("GetMe must query GraphQL whoami")
	}
	if op == nil || op.ID != 1 || op.Username != "mythic_admin" || !op.Admin {
		t.Fatalf("operator = %+v", op)
	}
	if op.CurrentOperation == nil || op.CurrentOperation.ID != 1 {
		t.Fatalf("current operation = %+v", op.CurrentOperation)
	}
}

func TestDownloadFileUsesDirectDownload(t *testing.T) {
	var sawLegacy bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/files/download") {
			sawLegacy = true
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/direct/download/file-uuid" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("mcp-smoke-test"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client, err := NewClient(&Config{
		ServerURL: srv.URL,
		APIToken:  "mtk_test",
		SSL:       false,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	data, err := client.DownloadFile(context.Background(), "file-uuid")
	if err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	if sawLegacy {
		t.Fatal("DownloadFile must not call /files/download")
	}
	if string(data) != "mcp-smoke-test" {
		t.Fatalf("body = %q", data)
	}
}
