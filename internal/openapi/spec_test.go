package openapi

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProjectSwaggerAndOpenAPI(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	writeProjectFixture(t, tempHome, "swaggerproj", "swagger.json", `{
  "baseUrl": "https://example.com",
  "authHeader": "Authorization",
  "authToken": "Bearer token"
}`)
	writeProjectFixture(t, tempHome, "openapiproj", "openapi.yaml", `{
  "baseUrl": "https://example.com"
}`)

	swaggerProject, err := LoadProject("swaggerproj")
	if err != nil {
		t.Fatalf("load swagger project: %v", err)
	}
	if swaggerProject.SpecType != "swagger" {
		t.Fatalf("expected swagger, got %s", swaggerProject.SpecType)
	}
	if len(swaggerProject.Operations) != 2 {
		t.Fatalf("expected 2 swagger operations, got %d", len(swaggerProject.Operations))
	}

	openapiProject, err := LoadProject("openapiproj")
	if err != nil {
		t.Fatalf("load openapi project: %v", err)
	}
	if openapiProject.SpecType != "openapi" {
		t.Fatalf("expected openapi, got %s", openapiProject.SpecType)
	}
	if len(openapiProject.Operations) != 2 {
		t.Fatalf("expected 2 openapi operations, got %d", len(openapiProject.Operations))
	}
}

func TestLoadProjectSwaggerWithInvalidNullTypeStillWorks(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	projectDir := filepath.Join(tempHome, ".openapi-cli", "apis", "broken-swagger")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	spec := `{
  "swagger": "2.0",
  "info": { "title": "Broken", "version": "1.0.0" },
  "paths": {
    "/biz/abnormal-logistic-record": {
      "post": {
        "tags": ["biz"],
        "summary": "Create record",
        "parameters": [{
          "name": "body",
          "in": "body",
          "required": true,
          "schema": {
            "type": "object",
            "properties": {
              "remark": { "type": "null" }
            }
          }
        }],
        "responses": {
          "200": { "description": "OK" }
        }
      }
    }
  }
}`
	if err := os.WriteFile(filepath.Join(projectDir, "api.json"), []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "setting.json"), []byte(`{"baseUrl":"https://example.com"}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	project, err := LoadProject("broken-swagger")
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	if len(project.Operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(project.Operations))
	}
	if project.Operations[0].Path != "/biz/abnormal-logistic-record" {
		t.Fatalf("unexpected path: %s", project.Operations[0].Path)
	}
}

func TestSearchProjects(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	writeProjectFixture(t, tempHome, "erp", "openapi.yaml", `{"baseUrl":"https://example.com"}`)
	results, err := SearchProjects("erp", "users", "lookup")
	if err != nil {
		t.Fatalf("search projects: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Path != "/users/{id}" {
		t.Fatalf("unexpected result path: %s", results[0].Path)
	}
}

func TestRenderOperationDoc(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	writeProjectFixture(t, tempHome, "erp", "swagger.json", `{"baseUrl":"https://example.com"}`)
	project, err := LoadProject("erp")
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	op, err := FindOperation(project, "/users", "POST")
	if err != nil {
		t.Fatalf("find operation: %v", err)
	}
	markdown, err := RenderOperationDoc(op, "markdown")
	if err != nil {
		t.Fatalf("render markdown: %v", err)
	}
	if !strings.Contains(markdown, "# POST /users") {
		t.Fatalf("unexpected markdown: %s", markdown)
	}
	openapiDoc, err := RenderOperationDoc(op, "openapi")
	if err != nil {
		t.Fatalf("render openapi: %v", err)
	}
	if !strings.Contains(openapiDoc, "openapi: 3.0.0") {
		t.Fatalf("unexpected openapi doc: %s", openapiDoc)
	}
	swaggerDoc, err := RenderOperationDoc(op, "swagger")
	if err != nil {
		t.Fatalf("render swagger: %v", err)
	}
	if !strings.Contains(swaggerDoc, "swagger: \"2.0\"") {
		t.Fatalf("unexpected swagger doc: %s", swaggerDoc)
	}
}

func TestCallOperation(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	writeProjectFixture(t, tempHome, "erp", "openapi.yaml", `{
  "baseUrl": "http://placeholder"
}`)
	project, err := LoadProject("erp")
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	op, err := FindOperation(project, "/users", "POST")
	if err != nil {
		t.Fatalf("find operation: %v", err)
	}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("page") != "1" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		return &http.Response{
			StatusCode: http.StatusCreated,
			Status:     "201 Created",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
		}, nil
	})}

	project.Config.BaseURL = "https://example.com"
	project.Config.AuthHeader = "Authorization"
	project.Config.AuthToken = "Bearer test"
	result, err := CallOperation(project, op, CallOptions{
		QueryJSON: `{"page":1}`,
		BodyJSON:  `{"name":"alice"}`,
		Client:    client,
	})
	if err != nil {
		t.Fatalf("call operation: %v", err)
	}
	if result.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status: %d", result.StatusCode)
	}
	if got := FormatResponseBody(result.Body); !strings.Contains(got, `"ok": true`) {
		t.Fatalf("unexpected body: %s", got)
	}
}

func TestBuildRequestAndRenderCurlCommand(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	writeProjectFixture(t, tempHome, "erp", "openapi.yaml", `{
  "baseUrl": "https://example.com"
}`)
	project, err := LoadProject("erp")
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	op, err := FindOperation(project, "/users", "POST")
	if err != nil {
		t.Fatalf("find operation: %v", err)
	}
	project.Config.AuthHeader = "Authorization"
	project.Config.AuthToken = "Bearer demo"

	req, err := BuildRequest(project, op, CallOptions{
		QueryJSON: `{"page":1,"tag":["a","b"]}`,
		BodyJSON:  `{"name":"alice"}`,
	})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	curl := RenderCurlCommand(req)
	if !strings.Contains(curl, `curl -X "POST"`) {
		t.Fatalf("unexpected curl method: %s", curl)
	}
	if !strings.Contains(curl, `-H "Authorization: Bearer demo"`) {
		t.Fatalf("unexpected curl auth header: %s", curl)
	}
	if !strings.Contains(curl, `-H "Content-Type: application/json"`) {
		t.Fatalf("unexpected curl content-type: %s", curl)
	}
	if !strings.Contains(curl, `--data-raw "{\"name\":\"alice\"}"`) {
		t.Fatalf("unexpected curl body: %s", curl)
	}
	if !strings.Contains(curl, `https://example.com/users?page=1&tag=a&tag=b`) &&
		!strings.Contains(curl, `https://example.com/users?tag=a&tag=b&page=1`) {
		t.Fatalf("unexpected curl url: %s", curl)
	}
}

func TestBuildRequestSupportsColonPathParams(t *testing.T) {
	project := &Project{
		Name: "erp",
		Config: Config{
			BaseURL: "https://example.com",
		},
	}
	op := &Operation{
		Method: "GET",
		Path:   "/user-order/get/:orderId",
	}
	req, err := BuildRequest(project, op, CallOptions{
		QueryJSON: `{"orderId":"A-1001","pageNum":"1"}`,
	})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if req.URL.Path != "/user-order/get/A-1001" {
		t.Fatalf("unexpected path: %s", req.URL.Path)
	}
	if req.URL.Query().Get("orderId") != "" {
		t.Fatalf("path parameter should not remain in query: %s", req.URL.RawQuery)
	}
	if req.URL.Query().Get("pageNum") != "1" {
		t.Fatalf("unexpected query: %s", req.URL.RawQuery)
	}
}

func TestBuildRequestRequiresPathParams(t *testing.T) {
	project := &Project{
		Name: "erp",
		Config: Config{
			BaseURL: "https://example.com",
		},
	}
	op := &Operation{
		Method: "GET",
		Path:   "/user-order/get/:orderId",
	}
	_, err := BuildRequest(project, op, CallOptions{
		QueryJSON: `{"pageNum":"1"}`,
	})
	if err == nil || !strings.Contains(err.Error(), "missing path parameter") {
		t.Fatalf("expected path parameter error, got %v", err)
	}
}

func TestResponseHint(t *testing.T) {
	project := &Project{Name: "erp"}
	op := &Operation{Method: "GET", Path: "/users"}

	unauthorized := ResponseHint(project, op, &CallResult{StatusCode: http.StatusUnauthorized})
	if !strings.Contains(unauthorized, "authHeader/authToken") {
		t.Fatalf("unexpected 401 hint: %s", unauthorized)
	}

	notFound := ResponseHint(project, op, &CallResult{StatusCode: http.StatusNotFound})
	if !strings.Contains(notFound, "baseUrl") {
		t.Fatalf("unexpected 404 hint: %s", notFound)
	}

	serverErr := ResponseHint(project, op, &CallResult{StatusCode: http.StatusInternalServerError})
	if !strings.Contains(serverErr, "traceId") {
		t.Fatalf("unexpected 500 hint: %s", serverErr)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func TestConfigReadWrite(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	projectDir := filepath.Join(tempHome, ".openapi-cli", "apis", "erp")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, err := WriteConfig("erp", Config{
		BaseURL:    "https://example.com",
		AuthHeader: "Authorization",
		AuthToken:  "Bearer abc",
		UpdateURL:  "https://example.com/swagger.json",
	})
	if err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, _, err := ReadConfig("erp")
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if cfg.AuthToken != "Bearer abc" {
		t.Fatalf("unexpected auth token: %s", cfg.AuthToken)
	}
	if cfg.UpdateURL != "https://example.com/swagger.json" {
		t.Fatalf("unexpected update url: %s", cfg.UpdateURL)
	}
}

func TestFindOperationInfersMethodWhenPathIsUnique(t *testing.T) {
	project := &Project{
		Operations: []Operation{{Path: "/users", Method: "GET"}},
	}
	op, err := FindOperation(project, "/users", "")
	if err != nil {
		t.Fatalf("expected inferred method, got %v", err)
	}
	if op.Method != "GET" {
		t.Fatalf("unexpected inferred method: %s", op.Method)
	}
}

func TestFindOperationRequiresMethodWhenPathIsAmbiguous(t *testing.T) {
	project := &Project{
		Operations: []Operation{
			{Path: "/users", Method: "GET"},
			{Path: "/users", Method: "POST"},
		},
	}
	_, err := FindOperation(project, "/users", "")
	if err == nil || !strings.Contains(err.Error(), "multiple methods found") {
		t.Fatalf("expected ambiguous method error, got %v", err)
	}
}

func TestCreateProjectFromLocalFile(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	sourcePath := filepath.Join(tempHome, "swagger.json")
	specData, err := os.ReadFile(filepath.Join("testdata", "swagger.json"))
	if err != nil {
		t.Fatalf("read source fixture: %v", err)
	}
	if err := os.WriteFile(sourcePath, specData, 0o644); err != nil {
		t.Fatalf("write source fixture: %v", err)
	}
	configPath, specPath, err := CreateProject("erp", CreateProjectOptions{Source: sourcePath})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("stat config path: %v", err)
	}
	written, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read written spec: %v", err)
	}
	if string(written) != string(specData) {
		t.Fatalf("unexpected written spec content")
	}
	cfg, _, err := ReadConfig("erp")
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if cfg.UpdateURL != "" {
		t.Fatalf("expected empty update url, got %s", cfg.UpdateURL)
	}
}

func TestCreateProjectFromURLAndSync(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	originalClient := specHTTPClient
	defer func() {
		specHTTPClient = originalClient
	}()

	payloads := map[string]string{
		"https://example.com/spec.json":    `{"swagger":"2.0","info":{"title":"A","version":"1.0"},"paths":{}}`,
		"https://example.com/spec-v2.json": `{"swagger":"2.0","info":{"title":"B","version":"1.0"},"paths":{}}`,
	}
	specHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, ok := payloads[r.URL.String()]
		if !ok {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Status:     "404 Not Found",
				Body:       io.NopCloser(strings.NewReader("not found")),
				Header:     http.Header{},
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}

	_, specPath, err := CreateProject("erp", CreateProjectOptions{Source: "https://example.com/spec.json"})
	if err != nil {
		t.Fatalf("create project from url: %v", err)
	}
	cfg, _, err := ReadConfig("erp")
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if cfg.UpdateURL != "https://example.com/spec.json" {
		t.Fatalf("unexpected update url: %s", cfg.UpdateURL)
	}

	cfg.UpdateURL = "https://example.com/spec-v2.json"
	if _, err := WriteConfig("erp", cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	updatedPath, err := SyncProject("erp")
	if err != nil {
		t.Fatalf("sync project: %v", err)
	}
	if updatedPath != specPath {
		t.Fatalf("unexpected sync path: %s", updatedPath)
	}
	updatedData, err := os.ReadFile(updatedPath)
	if err != nil {
		t.Fatalf("read synced spec: %v", err)
	}
	if !strings.Contains(string(updatedData), `"title":"B"`) {
		t.Fatalf("unexpected synced spec: %s", string(updatedData))
	}
}

func TestCreateProjectWithInitialConfigOptions(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	sourcePath := filepath.Join(tempHome, "openapi.yaml")
	specData, err := os.ReadFile(filepath.Join("testdata", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read source fixture: %v", err)
	}
	if err := os.WriteFile(sourcePath, specData, 0o644); err != nil {
		t.Fatalf("write source fixture: %v", err)
	}
	_, _, err = CreateProject("erp", CreateProjectOptions{
		Source:     sourcePath,
		BaseURL:    "https://api.example.com",
		AuthHeader: "Authorization",
		AuthToken:  "Bearer seeded",
		UpdateURL:  "https://mirror.example.com/openapi.yaml",
		SpecType:   "openapi",
	})
	if err != nil {
		t.Fatalf("create project with options: %v", err)
	}
	cfg, _, err := ReadConfig("erp")
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if cfg.BaseURL != "https://api.example.com" {
		t.Fatalf("unexpected base url: %s", cfg.BaseURL)
	}
	if cfg.AuthHeader != "Authorization" {
		t.Fatalf("unexpected auth header: %s", cfg.AuthHeader)
	}
	if cfg.AuthToken != "Bearer seeded" {
		t.Fatalf("unexpected auth token: %s", cfg.AuthToken)
	}
	if cfg.UpdateURL != "https://mirror.example.com/openapi.yaml" {
		t.Fatalf("unexpected update url: %s", cfg.UpdateURL)
	}
	if cfg.SpecType != "openapi" {
		t.Fatalf("unexpected spec type: %s", cfg.SpecType)
	}
}

func writeProjectFixture(t *testing.T, home, project, fixtureName, configJSON string) {
	t.Helper()
	projectDir := filepath.Join(home, ".openapi-cli", "apis", project)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	specData, err := os.ReadFile(filepath.Join("testdata", fixtureName))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	targetSpec := filepath.Join(projectDir, "api."+strings.TrimPrefix(filepath.Ext(fixtureName), "."))
	if err := os.WriteFile(targetSpec, specData, 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "setting.json"), []byte(configJSON), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
