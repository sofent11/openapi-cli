package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentCall(t *testing.T) {
	const secret = "session-example-abc"
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TEST_AUTH_VALUE", "Bearer "+secret)
	calls := 0
	status := 200
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("X-Custom") != "Bearer "+secret {
			t.Error("missing credential")
		}
		w.Header().Set("Set-Cookie", "id="+secret)
		w.WriteHeader(status)
		if status != 204 {
			fmt.Fprintf(w, `{"n":9007199254740993,"echo":"%s","nested":{"access_token":"issued-secret"}}`, secret)
		}
	}))
	defer srv.Close()
	spec := filepath.Join(t.TempDir(), "spec.json")
	if err := os.WriteFile(spec, []byte(`{"openapi":"3.0.3","info":{"title":"test","version":"1"},"paths":{"/tags":{"patch":{"responses":{"200":{"description":"OK"}}}}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (string, error) {
		root := NewRootCommand()
		var out, diagnostics bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&diagnostics)
		root.SetArgs(args)
		err := root.Execute()
		combined := out.String() + diagnostics.String() + fmt.Sprint(err)
		for _, s := range []string{secret, "issued-secret", "preview-password", "query-secret"} {
			if strings.Contains(combined, s) {
				t.Fatal("secret in command output")
			}
		}
		return out.String(), err
	}
	if _, err := run("config", "create", "-p", "t", spec, "--base-url", srv.URL, "--auth-header", "X-Custom", "--auth-token-env", "TEST_AUTH_VALUE"); err != nil {
		t.Fatal(err)
	}
	args := []string{"call", "-p", "t", "-i", "/tags", "--output", "json", "--fail-with-body", "-v", "-d", `{"password":"preview-password"}`, "-q", `{"api_key":"query-secret"}`}
	out, err := run(append(args, "--dry-run")...)
	if err != nil || calls != 0 || !json.Valid([]byte(out)) || !strings.Contains(out, "dryRun") {
		t.Fatal("invalid dry run", err)
	}
	for i, code := range []int{200, 204, 401, 403, 404, 422, 500} {
		status = code
		out, err = run(args...)
		var value struct {
			Status int
			Ok     bool
			Body   json.RawMessage
		}
		if e := json.Unmarshal([]byte(out), &value); e != nil {
			t.Fatal(e)
		}
		if calls != i+1 || value.Status != status || value.Ok != (status < 300) || (err != nil) != (status >= 400) {
			t.Fatalf("unexpected result for %d", code)
		}
		if code == 204 && string(value.Body) != "null" {
			t.Fatal("invalid empty body")
		}
		if code == 200 && !strings.Contains(string(value.Body), "9007199254740993") {
			t.Fatal("integer lost precision")
		}
	}
	srv.Close()
	out, err = run(args...)
	if err == nil || !strings.Contains(out, `"kind":"transport"`) {
		t.Fatal("missing network failure")
	}
	t.Setenv("TEST_AUTH_VALUE", "")
	out, err = run(args...)
	if err == nil || !strings.Contains(out, `"kind":"input"`) {
		t.Fatal("missing credential failure")
	}
}
