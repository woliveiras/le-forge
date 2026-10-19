package projectconfig

import (
	"github.com/woliveiras/le-forge/server/internal/storage/sqlstore"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterAndCreateIndependentWorktrees(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.Mkdir(repo, 0700); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		b, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %s", err, b)
		}
		return strings.TrimSpace(string(b))
	}
	run("init", "-q")
	run("-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "--allow-empty", "-qm", "initial")
	if err := os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(root, "state.db")
	s, err := sqlstore.OpenSQLite(t.Context(), db)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Migrate(t.Context()); err != nil {
		t.Fatal(err)
	}
	p, err := (Manager{Store: s}).Create(t.Context(), Input{Name: "Fixture", Path: repo}, "admin")
	if err != nil {
		t.Fatal(err)
	}
	oldRevision := p.Revision
	p, err = (Manager{Store: s}).SetExecutionPreference(t.Context(), p.ID, "local", p.Revision, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = (Manager{Store: s}).SetExecutionPreference(t.Context(), p.ID, "sandbox", oldRevision, "admin"); err == nil {
		t.Fatal("stale preference accepted")
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = sqlstore.OpenSQLite(t.Context(), db)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err = (Manager{Store: s}).Get(t.Context(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.ExecutionMode != "local" || p.Revision <= oldRevision {
		t.Fatal("preference not retained", p)
	}
	workspace := filepath.Join(root, "workspaces")
	if err = os.Mkdir(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	a, err := CreateWorktree(t.Context(), p, p.Head, workspace, "task-run-a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := CreateWorktree(t.Context(), p, p.Head, workspace, "task-run-b")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("shared workspace")
	}
	if err = os.WriteFile(filepath.Join(a, "change"), []byte("one"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(b, "change")); !os.IsNotExist(err) {
		t.Fatal("cross task write", err)
	}
	if data, err := os.ReadFile(filepath.Join(repo, "dirty.txt")); err != nil || string(data) != "keep" {
		t.Fatal("dirty source changed", err)
	}
	if _, err = CreateWorktree(t.Context(), p, p.Head, workspace, "task-run-a"); err == nil {
		t.Fatal("replaced existing worktree")
	}
	if Verify(t.Context(), p, "HEAD") == nil {
		t.Fatal("accepted moving ref")
	}
	if _, _, err = Inspect(t.Context(), workspace); err == nil {
		t.Fatal("accepted non repository")
	}
	if run("rev-parse", "HEAD") != p.Head {
		t.Fatal("source HEAD changed")
	}
}
