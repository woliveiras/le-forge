package mcpconfig

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/woliveiras/le-forge/server/internal/storage"
)

func TestDeleteReservesIdentityAndErasesCredentials(t *testing.T) {
	m := testManager(t)
	in := input()
	in.Credential = &Credential{Value: "synthetic-deletion-secret"}
	v, err := m.Put(t.Context(), in, 0, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Delete(t.Context(), in.ID, v.Revision, "admin"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	in.Enabled = false
	in.Credential = nil
	v, err = m.Put(t.Context(), in, v.Revision, "admin")
	if err != nil {
		t.Fatal(err)
	}
	stale, _ := m.load(t.Context(), in.ID)
	if err = m.Delete(t.Context(), in.ID, v.Revision-1, "admin"); !errors.Is(err, storage.ErrConflict) {
		t.Fatal(err)
	}
	m.pending = map[string]oauthPending{"same": {ID: in.ID}, "other": {ID: "other"}}
	if err = m.Delete(t.Context(), in.ID, v.Revision, "admin"); err != nil {
		t.Fatal(err)
	}
	if len(m.pending) != 1 || m.pending["other"].ID != "other" {
		t.Fatal(m.pending)
	}
	restarted := New(m.Store, m.Box, nil)
	if _, err = restarted.Get(t.Context(), in.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatal(err)
	}
	views, err := restarted.List(t.Context())
	if err != nil || len(views) != 0 {
		t.Fatal(views, err)
	}
	if _, err = restarted.Put(t.Context(), in, 0, "admin"); !errors.Is(err, storage.ErrConflict) {
		t.Fatal("ID reused", err)
	}
	if _, err = restarted.save(t.Context(), stale, stale.Revision, "admin", "test"); !errors.Is(err, storage.ErrConflict) {
		t.Fatal("stale writer resurrected connection", err)
	}
	setting, err := m.Store.GetSetting(t.Context(), prefix+in.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"sealed", "oauth", "example.com", "synthetic-deletion-secret"} {
		if strings.Contains(strings.ToLower(string(setting.Value)), forbidden) {
			t.Fatal("tombstone retains configuration", forbidden)
		}
	}
}

func TestDeleteFailureAndOAuthPrerequisites(t *testing.T) {
	m := testManager(t)
	in := input()
	in.Enabled = false
	v, err := m.Put(t.Context(), in, 0, "admin")
	if err != nil {
		t.Fatal(err)
	}
	original := m.Store
	m.Store = failedWrites{original}
	if err = m.Delete(t.Context(), in.ID, v.Revision, "admin"); err == nil {
		t.Fatal("write failure hidden")
	}
	m.Store = original
	if _, err = m.Get(t.Context(), in.ID); err != nil {
		t.Fatal(err)
	}
	r, _ := m.load(t.Context(), in.ID)
	r.OAuth = &OAuthView{Status: "authorized"}
	v, err = m.save(t.Context(), r, r.Revision, "admin", "test")
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Delete(t.Context(), in.ID, v.Revision, "admin"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	r, _ = m.load(t.Context(), in.ID)
	r.OAuth.Status = "revoked"
	r.OAuth.Revocation = "pending"
	v, _ = m.save(t.Context(), r, r.Revision, "admin", "test")
	if err = m.Delete(t.Context(), in.ID, v.Revision, "admin"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	r, _ = m.load(t.Context(), in.ID)
	r.OAuth.Revocation = "unsupported"
	v, _ = m.save(t.Context(), r, r.Revision, "admin", "test")
	if err = m.Delete(t.Context(), in.ID, v.Revision, "admin"); err != nil {
		t.Fatal(err)
	}
	if err = m.Delete(context.Background(), "../bad", 1, "admin"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}

func TestDeletionWinsAgainstInflightDiscovery(t *testing.T) {
	m := testManager(t)
	in := input()
	v, err := m.Put(t.Context(), in, 0, "admin")
	if err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	m.Discover = func(context.Context, string, string) ([]Tool, error) { close(started); <-release; return []Tool{}, nil }
	done := make(chan error, 1)
	go func() { _, err := m.Refresh(t.Context(), in.ID, "operator"); done <- err }()
	<-started
	in.Enabled = false
	v, err = m.Put(t.Context(), in, v.Revision, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Delete(t.Context(), in.ID, v.Revision, "admin"); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err = <-done; !errors.Is(err, storage.ErrConflict) {
		t.Fatal("stale discovery did not conflict", err)
	}
	if _, err = m.Get(t.Context(), in.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatal("connection resurrected", err)
	}
}
