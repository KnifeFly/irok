package kiro

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aiclient2api/internal/config"
	"aiclient2api/internal/pool"
	"aiclient2api/internal/provider"
)

func TestCreateMessageWithMockKiroResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer access" {
			t.Fatalf("missing auth header: %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`:message-typeevent{"content":"hello from kiro"}`))
	}))
	defer server.Close()

	dir := t.TempDir()
	credPath := filepath.Join(dir, "cred.json")
	creds := Credentials{
		AccessToken:  "access",
		RefreshToken: "refresh",
		AuthMethod:   "social",
		Region:       "us-east-1",
		ExpiresAt:    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}
	raw, _ := json.Marshal(creds)
	if err := os.WriteFile(credPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	pools, err := pool.NewStore(filepath.Join(dir, "pools.toml"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = pools.Upsert(pool.Node{ID: "node", Name: "node", CredentialPath: credPath, Enabled: true, Healthy: true})
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.Kiro.BaseURLTemplate = server.URL
	service := New(cfg, pools, slog.Default())
	resp, err := service.CreateMessage(context.Background(), provider.MessageRequest{
		Model: "claude-sonnet-4-5",
		Messages: []provider.Message{{
			Role:    "user",
			Content: "hello",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Content) != 1 || resp.Content[0].Text != "hello from kiro" {
		t.Fatalf("unexpected response: %#v", resp.Content)
	}
}

func TestParseKiroResponseToolUse(t *testing.T) {
	content, tools := parseKiroResponse([]byte(`event{"name":"do_work","toolUseId":"tool_1"}event{"toolUseId":"tool_1","input":{"ok":true}}`))
	if content != "" {
		t.Fatalf("content = %q", content)
	}
	if len(tools) != 1 || tools[0].Name != "do_work" || tools[0].ID != "tool_1" {
		t.Fatalf("unexpected tools: %#v", tools)
	}
}
