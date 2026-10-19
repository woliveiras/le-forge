package api_test

import (
	"encoding/json"
	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/projectconfig"
	"github.com/woliveiras/le-forge/server/internal/taskconfig"
	"github.com/woliveiras/le-forge/server/internal/taskrun"
	"github.com/woliveiras/le-forge/server/internal/workspace"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProjectTaskWorktreeThroughAPI(t *testing.T) {
	srv, _ := setupApp(t, nil)
	repo := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "--allow-empty", "-qm", "initial"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatal(err, string(b))
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "uncommitted"), []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(projectconfig.Input{Name: "Project", Path: repo})
	for _, tok := range []string{viewerTok, operatorTok} {
		if c := call(t, srv, "POST", "/api/v1/projects", tok, "application/json", string(body), nil); c != 403 {
			t.Fatal(c)
		}
	}
	var p projectconfig.Project
	if c := call(t, srv, "POST", "/api/v1/projects", adminTok, "application/json", string(body), &p); c != 201 {
		t.Fatal(c)
	}
	var ps []projectconfig.Project
	if c := call(t, srv, "GET", "/api/v1/projects", viewerTok, "", "", &ps); c != 200 || len(ps) != 1 || ps[0].ID != p.ID {
		t.Fatal(c, ps)
	}
	if c := call(t, srv, "POST", "/api/v1/projects", adminTok, "application/json", `{"name":"x","path":"relative"}`, nil); c != 400 {
		t.Fatal(c)
	}
	var workflow domain.Graph
	if err := json.Unmarshal([]byte(taskGraph), &workflow); err != nil {
		t.Fatal(err)
	}
	workflow.Spec.Nodes[0].Workspace.Mode = domain.WorkspaceWorktree
	graphBody, _ := json.Marshal(workflow)
	call(t, srv, "POST", "/api/v1/graphs", adminTok, "application/json", string(graphBody), nil)
	var input taskconfig.Input
	if err := json.Unmarshal([]byte(taskBody), &input); err != nil {
		t.Fatal(err)
	}
	input.ProjectID = p.ID
	input.BaseCommit = p.Head
	input.ResultNodeID = input.NodeID
	b, _ := json.Marshal(input)
	var d taskconfig.Draft
	if c := call(t, srv, "POST", "/api/v1/tasks", adminTok, "application/json", string(b), &d); c != 201 {
		t.Fatal(c)
	}
	var review taskrun.Review
	if c := call(t, srv, "GET", "/api/v1/tasks/"+d.ID+"/review", adminTok, "", "", &review); c != 200 || !review.CanStart || review.Project == nil {
		t.Fatal(c, review)
	}
	if !strings.Contains(review.WorkspacePath, "task-run-") {
		t.Fatal(review.WorkspacePath)
	}
	if c := taskStart(t, srv.URL, d.ID, adminTok, d.Revision); c != 202 {
		t.Fatal(c)
	}
	waitFor(t, 5*time.Second, "project task", func() bool {
		var v taskrun.View
		call(t, srv, "GET", "/api/v1/tasks/"+d.ID+"/progress", adminTok, "", "", &v)
		return v.State == "succeeded" && v.WorkspacePath == review.WorkspacePath
	})
	if _, err := os.Stat(filepath.Join(review.WorkspacePath, ".git")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(review.WorkspacePath, "new-file"), []byte("task change"), 0600); err != nil {
		t.Fatal(err)
	}
	var inspection workspace.Inspection
	endpoint := "/api/v1/graphs/" + taskrun.ExecutionID(d.ID) + "/git/status"
	if code := call(t, srv, "GET", endpoint, viewerTok, "", "", &inspection); code != 200 || !inspection.Repository || !strings.Contains(inspection.Content, "new-file") {
		t.Fatalf("task inspection: %d %+v", code, inspection)
	}

	var progress taskrun.View
	if code := call(t, srv, "GET", "/api/v1/tasks/"+d.ID+"/progress", viewerTok, "", "", &progress); code != 200 || len(progress.Workspaces) != 1 {
		t.Fatal(code, progress)
	}
	scope := progress.Workspaces[0]
	if scope.InspectionNode != input.NodeID || scope.Path == review.WorkspacePath || scope.Branch == "" {
		t.Fatal(scope)
	}
	if err := os.WriteFile(filepath.Join(scope.Path, "agent-file"), []byte("agent change"), 0600); err != nil {
		t.Fatal(err)
	}
	if code := call(t, srv, "GET", endpoint+"?node="+input.NodeID+"&replica=0", viewerTok, "", "", &inspection); code != 200 || !strings.Contains(inspection.Content, "agent-file") || strings.Contains(inspection.Content, "new-file") {
		t.Fatal(code, inspection)
	}
	if code := call(t, srv, "GET", endpoint+"?node=missing&replica=0", viewerTok, "", "", nil); code == 200 {
		t.Fatal("unknown scope accepted")
	}
	// A modified .git pointer must not expose a different repository.
	if err := os.WriteFile(filepath.Join(review.WorkspacePath, ".git"), []byte("gitdir: /outside/fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if code := call(t, srv, "GET", endpoint, viewerTok, "", "", nil); code == 200 {
		t.Fatal("redirected task metadata accepted")
	}
	if b, err := os.ReadFile(filepath.Join(repo, "uncommitted")); err != nil || string(b) != "preserve" {
		t.Fatal("dirty source changed", err)
	}
}
