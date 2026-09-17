# mythic-sdk-go (Mythic 4 patch)

Local fork of [nbaertsch/mythic-sdk-go](https://github.com/nbaertsch/mythic-sdk-go) for Mythic 4.x:

- `Authorization: Bearer` for API tokens and JWTs (no `apitoken` header)
- Unprefixed action routes (no `/api/v1.4`)
- `createTask` GraphQL with `callback_display_id`
- camelCase Hasura actions (`rebuildPayload`, `configCheck`, `redirectRules`, `callbackgraphedgeAdd` / `Remove`)

Used via `replace` from `references/Mythic-MCP`.
