---
name: openapi-cli
description: Use a local `openapi-cli` binary to import Swagger 2.0 or OpenAPI 3.x specs, manage API projects, search endpoints, inspect endpoint docs, and prepare or execute HTTP calls from the terminal. Use when Codex needs to help a user work from `swagger.json`, `openapi.json`, `openapi.yaml`, or a remote spec URL through this CLI, especially for `config create`, `config sync`, `list`, `search`, `doc`, and `call` workflows.
---

# OpenAPI CLI

Help the user operate an `openapi-cli` binary installed in `PATH` as a terminal-first API explorer and caller. Prefer short, runnable commands and keep the workflow anchored on an existing or newly imported project.

## Start Here

1. Confirm `openapi-cli` is available from `PATH`.
2. If it is missing, guide the user to download the correct archive from the GitHub Releases page, unpack the binary, place it in a directory already in `PATH` such as `/usr/local/bin`, and make it executable.
3. If the user has not imported a spec yet, create a project first.
4. If the user does not know the exact endpoint path, search before using `doc` or `call`.
5. Before a real request, verify `baseUrl`, auth settings, method selection, and whether parameters belong in query or request body.

## Core Workflow

### 1. Import or update a project

Use `config create` when the user starts from a local file or a remote URL.

```bash
openapi-cli config create -p erp ./openapi.yaml --base-url https://api.example.com
openapi-cli config create -p erp https://example.com/openapi.json \
  --base-url https://api.example.com \
  --auth-header Authorization \
  --auth-token 'Bearer xxx'
```

Use `config sync -p <project>` only when the project already has `updateUrl`.

### 2. Discover projects or categories

Use `list` to understand what is already imported.

```bash
openapi-cli list
openapi-cli list -p erp
```

Without `-p`, list projects. With `-p`, list categories or tags in that project.

### 3. Find the right endpoint

Use `search` whenever the user gives a keyword instead of an exact path.

```bash
openapi-cli search -p erp -i order
openapi-cli search -p erp -c users -i /users
```

Search matches `summary`, `description`, `path`, `operationId`, and `tags`. Treat the returned path as the source of truth for later `doc` or `call` commands.

### 4. Inspect endpoint documentation

Use `doc` after the path is known.

```bash
openapi-cli doc -p erp -i /users/{id}
openapi-cli doc -p erp -i /users -m POST -t markdown
```

Use `-m` when one path has multiple methods. Prefer `-t markdown` unless the user explicitly wants normalized OpenAPI or Swagger-shaped output.

### 5. Prepare or execute a request

Use `call` only after confirming the project config and the request shape.

```bash
openapi-cli call -p erp -i /users/{id} -m GET -q '{"id":"1"}' -v
openapi-cli call -p erp -i /users -m POST -d '{"name":"alice"}' -v
```

Use `-q` for query-string JSON objects and `-d` for JSON request bodies. Prefer `-v` first so the user can inspect the generated `curl` before relying on the result.

## Method And Parameter Rules

- Omit `-m` only when the path resolves to exactly one HTTP method.
- Add `-m` immediately if the CLI reports multiple methods for the same path.
- Put path placeholder values such as `/users/{id}` into the JSON passed with `-q` unless the CLI already documents a different pattern for that endpoint.
- Ensure `baseUrl` is present before `call`.
- Ensure both `authHeader` and `authToken` are present when the API requires authentication.

## Config Management

Use these commands for targeted inspection or repair:

```bash
openapi-cli config -p erp
openapi-cli config get -p erp baseUrl
openapi-cli config set -p erp baseUrl https://api.example.com
openapi-cli config set -p erp authHeader Authorization
openapi-cli config set -p erp authToken 'Bearer xxx'
```

If the user reports stale docs, check `updateUrl` and then run `config sync`.

## Response Style

- Prefer concrete commands over abstract explanations.
- If the user gives an inexact endpoint name, search first instead of guessing.
- If the user wants to understand an endpoint, use `doc` before `call`.
- If a call may have side effects, show the `-v` form first.
- If the task needs exact flags, sample commands, or error-specific recovery, read [references/command-recipes.md](./references/command-recipes.md).
