package pool

import (
	"path/filepath"
	"testing"
)

func TestPoolSelectSkipsDisabledAndUnhealthy(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "pools.toml"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Upsert(Node{ID: "disabled", Name: "disabled", CredentialPath: "disabled.json", Enabled: false, Healthy: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Upsert(Node{ID: "bad", Name: "bad", CredentialPath: "bad.json", Enabled: true, Healthy: false, FailureCount: 3, LastError: "bad"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Upsert(Node{ID: "good", Name: "good", CredentialPath: "good.json", Enabled: true, Healthy: true})
	if err != nil {
		t.Fatal(err)
	}
	node, err := store.Select()
	if err != nil {
		t.Fatal(err)
	}
	if node.ID != "good" {
		t.Fatalf("selected %q, want good", node.ID)
	}
}

func TestMarkFailureEventuallyUnhealthy(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "pools.toml"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Upsert(Node{ID: "node", Name: "node", CredentialPath: "node.json", Enabled: true, Healthy: true})
	if err != nil {
		t.Fatal(err)
	}
	store.MarkFailure("node", "one", nil)
	store.MarkFailure("node", "two", nil)
	store.MarkFailure("node", "three", nil)
	node, ok := store.Get("node")
	if !ok {
		t.Fatal("node missing")
	}
	if node.Healthy {
		t.Fatal("node should be unhealthy after max failures")
	}
}
