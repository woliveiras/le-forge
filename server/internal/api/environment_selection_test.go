//go:build !windows

package api_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woliveiras/le-forge/server/internal/config"
	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/environment"
)

func TestDiscoverReviewAcceptSaveBuild(t *testing.T) {
	root := t.TempDir()
	bin, log := fakeDaemonCLI(t)
	srv, _ := setupApp(t, func(c *config.Config) { c.Workspaces.Root = root; c.Isolation.Docker.Bin = bin })
	const graphDoc = `apiVersion: leforge.io/v1
kind: Graph
metadata: {name: selection}
spec:
  nodes: [{id: worker, runtime: {kind: echo}}]
`
	const base = "/api/v1/graphs/selection"
	if code := call(t, srv, "POST", "/api/v1/graphs", adminTok, "application/yaml", graphDoc, nil); code != 201 {
		t.Fatalf("create %d", code)
	}
	dir := filepath.Join(root, "selection")
	if err := os.MkdirAll(filepath.Join(dir, ".leforge"), 0755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, ".leforge/environment.yaml")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(source, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("image: alpine\nsetup: [echo ready]")
	var discovery environment.Discovery
	if code := call(t, srv, "GET", base+"/environment/discovery", viewerTok, "", "", &discovery); code != 200 {
		t.Fatalf("discover %d", code)
	}
	selection := environment.Selection{Source: discovery.Source, Path: discovery.Candidates[0].Path, SHA256: discovery.Candidates[0].SHA256}
	body, _ := json.Marshal(selection)
	if code := call(t, srv, "POST", base+"/environment/resolve", "", "application/json", string(body), nil); code != 401 {
		t.Fatalf("anonymous %d", code)
	}
	for _, bad := range []string{`{}`, `{"path":"../outside"}`, string(body) + ` {}`, `{"unknown":true}`} {
		if code := call(t, srv, "POST", base+"/environment/resolve", viewerTok, "application/json", bad, nil); code != 400 {
			t.Fatalf("invalid %s: %d", bad, code)
		}
	}
	var env domain.EnvironmentSpec
	if code := call(t, srv, "POST", base+"/environment/resolve", viewerTok, "application/json", string(body), &env); code != 200 || env.Image != "alpine" || env.Origin == nil {
		t.Fatalf("resolve %d %+v", code, env)
	}
	var saved struct {
		Graph domain.Graph `json:"graph"`
	}
	if code := call(t, srv, "GET", base, viewerTok, "", "", &saved); code != 200 || saved.Graph.Spec.Environment != nil {
		t.Fatalf("preview mutated graph %d %+v", code, saved)
	}
	saved.Graph.Spec.Environment = &env
	graphJSON, _ := json.Marshal(saved.Graph)
	if code := callIfMatch(t, srv, "PUT", base, viewerTok, string(graphJSON), 1, nil); code != 403 {
		t.Fatalf("viewer save %d", code)
	}
	write("image: debian")
	if code := callIfMatch(t, srv, "PUT", base, adminTok, string(graphJSON), 1, nil); code != 400 {
		t.Fatalf("stale review save %d", code)
	}
	write("image: alpine\nsetup: [echo ready]")
	if code := callIfMatch(t, srv, "PUT", base, adminTok, string(graphJSON), 1, &saved); code != 200 {
		t.Fatalf("accept save %d", code)
	}
	if saved.Graph.Spec.Environment.Origin.SHA256 != selection.SHA256 || saved.Graph.Metadata.Version != 2 {
		t.Fatalf("origin/version not persisted %+v", saved)
	}
	if code := callIfMatch(t, srv, "PUT", base, adminTok, string(graphJSON), 1, nil); code != 409 {
		t.Fatalf("concurrent save %d", code)
	}
	var state envView
	if code := call(t, srv, "GET", base+"/environment", viewerTok, "", "", &state); code != 200 || state.State != "absent" {
		t.Fatalf("accept started build %d %+v", code, state)
	}
	write("image: debian")
	if code := call(t, srv, "POST", base+"/environment/build", operatorTok, "application/json", `{}`, nil); code != 400 {
		t.Fatalf("stale build %d", code)
	}
	if data, err := os.ReadFile(log); err == nil && strings.TrimSpace(string(data)) != "" {
		t.Fatalf("Docker called before validation: %s", data)
	}
	write("image: alpine\nsetup: [echo ready]")
	if code := call(t, srv, "POST", base+"/environment/build", operatorTok, "application/json", `{}`, &state); code != 202 {
		t.Fatalf("build %d %+v", code, state)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		call(t, srv, "GET", base+"/environment", viewerTok, "", "", &state)
		if state.State != "building" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if state.State != "ready" {
		t.Fatalf("build %+v", state)
	}
	write("image: debian")
	if code := call(t, srv, "POST", base+"/environment/build", operatorTok, "application/json", `{}`, nil); code != 400 {
		t.Fatalf("cached build bypassed validation %d", code)
	}
}

func TestRepositoryContextAcceptanceAndStaleBuild(t *testing.T) {
	root := t.TempDir()
	bin, log := fakeDaemonCLI(t)
	srv, _ := setupApp(t, func(c *config.Config) { c.Workspaces.Root = root; c.Isolation.Docker.Bin = bin })
	const base = "/api/v1/graphs/context"
	const doc = "apiVersion: leforge.io/v1\nkind: Graph\nmetadata: {name: context}\nspec:\n  nodes: [{id: worker, runtime: {kind: echo}}]\n"
	if code := call(t, srv, "POST", "/api/v1/graphs", adminTok, "application/yaml", doc, nil); code != 201 {
		t.Fatalf("create %d", code)
	}
	dir := filepath.Join(root, "context")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	write := func(name, value string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("Dockerfile", "FROM alpine\nCOPY app /app\n")
	write("app", "one")
	var discovery environment.Discovery
	if code := call(t, srv, "GET", base+"/environment/discovery", viewerTok, "", "", &discovery); code != 200 {
		t.Fatalf("discover %d", code)
	}
	selection, _ := json.Marshal(environment.Selection{Source: discovery.Source, Path: discovery.Candidates[0].Path, SHA256: discovery.Candidates[0].SHA256})
	var env domain.EnvironmentSpec
	if code := call(t, srv, "POST", base+"/environment/resolve", viewerTok, "application/json", string(selection), &env); code != 200 || env.Context != "." || env.Origin == nil || env.Origin.ContextSHA256 == "" {
		t.Fatalf("resolve %d %+v", code, env)
	}
	var saved struct {
		Graph domain.Graph `json:"graph"`
	}
	if code := call(t, srv, "GET", base, viewerTok, "", "", &saved); code != 200 {
		t.Fatalf("get %d", code)
	}
	saved.Graph.Spec.Environment = &env
	body, _ := json.Marshal(saved.Graph)
	write("app", "changed")
	if code := callIfMatch(t, srv, "PUT", base, adminTok, string(body), 1, nil); code != 400 {
		t.Fatalf("stale context save %d", code)
	}
	write("app", "one")
	if code := callIfMatch(t, srv, "PUT", base, adminTok, string(body), 1, &saved); code != 200 || saved.Graph.Spec.Environment.Origin.ContextSHA256 != env.Origin.ContextSHA256 {
		t.Fatalf("save %d %+v", code, saved)
	}
	write("app", "changed after save")
	if code := call(t, srv, "POST", base+"/environment/build", operatorTok, "application/json", `{}`, nil); code != 400 {
		t.Fatalf("stale context build %d", code)
	}
	if data, err := os.ReadFile(log); err == nil && strings.TrimSpace(string(data)) != "" {
		t.Fatalf("Docker called before context validation: %s", data)
	}
}
