# OpenAPI CLI Command Recipes

Use this file when the task needs exact command shapes, setup details, or error recovery patterns.

## Quick Recipes

### Install from GitHub Releases

```bash
openapi-cli help
```

Default assumption: install `openapi-cli` from the repository's GitHub Releases page, unpack the archive for the user's platform, copy `openapi-cli` into a directory in `PATH` such as `/usr/local/bin`, run `chmod +x` if needed, then verify with `openapi-cli help`.

### Create a project from a local spec

```bash
openapi-cli config create -p erp ./openapi.yaml \
  --base-url https://api.example.com
```

### Create a project from a remote spec

```bash
openapi-cli config create -p erp https://example.com/openapi.json \
  --base-url https://api.example.com \
  --auth-header Authorization \
  --auth-token 'Bearer xxx'
```

If the source is a URL, `updateUrl` is recorded automatically.

### List imported projects and categories

```bash
openapi-cli list
openapi-cli list -p erp
```

### Search for endpoints

```bash
openapi-cli search -i login
openapi-cli search -p erp -i lookup
openapi-cli search -p erp -c users -i /users
```

### Show endpoint documentation

```bash
openapi-cli doc -p erp -i /users/{id}
openapi-cli doc -p erp -i /users -m POST -t markdown
openapi-cli doc -p erp -i /users -m POST -t openapi
openapi-cli doc -p erp -i /users -m POST -t swagger
```

### Execute a request

```bash
openapi-cli call -p erp -i /users/{id} -m GET -q '{"id":"1"}'
openapi-cli call -p erp -i /users -m POST -d '{"name":"alice"}'
openapi-cli call -p erp -i /biz/user-order/page -q '{"pageNum":"1","pageSize":"20"}' -v
```

## Decision Rules

### Choose between `search`, `doc`, and `call`

- Use `search` when the user knows a keyword, tag, or business name but not the exact path.
- Use `doc` when the path is known and the user needs request or response details.
- Use `call` when the path, method, parameters, and environment settings are already clear enough to execute a real request.

### Choose between `-q` and `-d`

- Use `-q` for query-string parameters.
- Use `-d` for JSON request bodies sent as `application/json`.
- Both values must be JSON objects.

### Choose whether to pass `-m`

- Skip `-m` only if one path has exactly one method.
- Pass `-m` when the same path supports multiple methods such as `GET` and `POST`.

## Config Keys

`setting.json` commonly contains:

```json
{
  "baseUrl": "https://api.example.com",
  "authHeader": "Authorization",
  "authToken": "Bearer xxx",
  "updateUrl": "https://example.com/openapi.json",
  "specType": "openapi"
}
```

- `baseUrl`: required for `call`
- `authHeader` and `authToken`: optional, but both are needed when auth is required
- `updateUrl`: required for `config sync`
- `specType`: usually let the CLI auto-detect it unless the user explicitly needs an override

## Data Layout

The default data root is:

```text
~/.openapi-cli/apis/
```

Each project uses a directory like:

```text
~/.openapi-cli/apis/erp/
├── api.json
└── setting.json
```

## Common Failures

### `required flag(s) "project" not set`

Add `-p <project>`.

### `interface not found`

Search first, then reuse the exact returned path.

```bash
openapi-cli search -p erp -i keyword
```

### `multiple methods found for ...`

Repeat the command with `-m GET`, `-m POST`, or the required method.

### `project baseUrl is empty`

Set `baseUrl` before calling the API.

```bash
openapi-cli config set -p erp baseUrl https://api.example.com
```

### Auth-related failures such as `Access token is empty`

Set both header name and token value.

```bash
openapi-cli config set -p erp authHeader Authorization
openapi-cli config set -p erp authToken 'Bearer xxx'
```

## Recommended Session Pattern

```bash
openapi-cli config create -p erp ./openapi.yaml --base-url https://api.example.com
openapi-cli search -p erp -i order
openapi-cli doc -p erp -i /biz/user-order/page
openapi-cli call -p erp -i /biz/user-order/page -q '{"pageNum":"1","pageSize":"20"}' -v
```
