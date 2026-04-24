package prompt

import (
	"path/filepath"
	"testing"
)

func TestApplyExactModelPrompt(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "prompts.toml"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Replace([]Rule{
		{Model: "*", Enabled: true, Mode: ModeAppend, Content: "fallback"},
		{Model: "claude-sonnet-4-5", Enabled: true, Mode: ModePrepend, Content: "exact"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := store.Apply("claude-sonnet-4-5", "request")
	if got != "exact\n\nrequest" {
		t.Fatalf("unexpected exact prompt: %q", got)
	}
	got = store.Apply("claude-haiku-4-5", "request")
	if got != "request\n\nfallback" {
		t.Fatalf("unexpected fallback prompt: %q", got)
	}
}

func TestOverrideAndOffModes(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "prompts.toml"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Replace([]Rule{
		{Model: "override", Enabled: true, Mode: ModeOverride, Content: "replacement"},
		{Model: "off", Enabled: true, Mode: ModeOff, Content: "ignored"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := store.Apply("override", "request"); got != "replacement" {
		t.Fatalf("override failed: %q", got)
	}
	if got := store.Apply("off", "request"); got != "request" {
		t.Fatalf("off failed: %q", got)
	}
}
