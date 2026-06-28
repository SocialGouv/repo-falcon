package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var req chatReq
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("bad request body: %v", err)
		}
		if req.Model != "test-model" || len(req.Messages) != 2 {
			t.Errorf("unexpected req: %+v", req)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"  Auth & Sessions \n"}}]}`))
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Model: "test-model", HTTP: srv.Client()}
	got, err := c.Complete(context.Background(), "sys", "user")
	if err != nil {
		t.Fatal(err)
	}
	if got != "Auth & Sessions" {
		t.Errorf("got %q, want trimmed content", got)
	}
}

func TestCompleteError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error":{"message":"model not found"}}`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Model: "x", HTTP: srv.Client()}
	if _, err := c.Complete(context.Background(), "s", "u"); err == nil {
		t.Error("expected error from error response")
	}
}

func TestFromEnvDefaultsLocal(t *testing.T) {
	t.Setenv("FALCON_LLM_BASE_URL", "")
	t.Setenv("OPENAI_BASE_URL", "")
	t.Setenv("FALCON_LLM_MODEL", "")
	t.Setenv("OPENAI_MODEL", "")
	c := FromEnv()
	if c.BaseURL != defaultBaseURL || c.Model != defaultModel {
		t.Errorf("defaults not local-first: base=%s model=%s", c.BaseURL, c.Model)
	}
}
