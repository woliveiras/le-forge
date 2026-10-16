//go:build !windows

package api_test

import (
	"context"
	"encoding/json"
	"github.com/woliveiras/le-forge/server/internal/config"
	"github.com/woliveiras/le-forge/server/internal/environment"
	"github.com/woliveiras/le-forge/server/internal/llm"
	"os"
	"path/filepath"
	"testing"
)

func TestEnvironmentPolicyAcrossReviewAndAgentApproval(t *testing.T) {
	root := t.TempDir()
	bin, log := fakeDaemonCLI(t)
	srv, app := setupApp(t, func(c *config.Config) {
		c.Workspaces.Root = root
		c.Isolation.Docker.Bin = bin
		c.Isolation.Docker.EnvironmentPolicy.SetupNetworks = []string{"none"}
	})
	doc := "apiVersion: leforge.io/v1\nkind: Graph\nmetadata: {name: policy}\nspec:\n  nodes: [{id: worker, runtime: {kind: echo}}]\n"
	if code := call(t, srv, "POST", "/api/v1/graphs", adminTok, "application/yaml", doc, nil); code != 201 {
		t.Fatal(code)
	}
	dir := filepath.Join(root, "policy", ".leforge")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "environment.yaml")
	for _, tc := range []struct {
		source string
		status int
	}{{"image: alpine", 400}, {"image: alpine\nnetwork: {setup: none}", 200}} {
		if err := os.WriteFile(path, []byte(tc.source), 0600); err != nil {
			t.Fatal(err)
		}
		var discovery environment.Discovery
		if code := call(t, srv, "GET", "/api/v1/graphs/policy/environment/discovery", viewerTok, "", "", &discovery); code != 200 {
			t.Fatal(code)
		}
		body, _ := json.Marshal(environment.Selection{Source: discovery.Source, Path: discovery.Candidates[0].Path, SHA256: discovery.Candidates[0].SHA256})
		if code := call(t, srv, "POST", "/api/v1/graphs/policy/environment/resolve", viewerTok, "application/json", string(body), nil); code != tc.status {
			t.Fatalf("review: %d", code)
		}
	}
	// A manually submitted definition is subject to exactly the same policy.
	manual := "apiVersion: leforge.io/v1\nkind: Graph\nmetadata: {name: manual}\nspec:\n  environment: {image: alpine}\n  nodes: [{id: worker, runtime: {kind: echo}}]\n"
	if code := call(t, srv, "POST", "/api/v1/graphs", adminTok, "application/yaml", manual, nil); code != 400 {
		t.Fatalf("manual: %d", code)
	}
	// Remove only the fixture file to make agent analysis applicable.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	provider := &proposalProvider{}
	app.Models.Add(llm.ProviderConfig{Name: "analysis"}, provider)
	if code := call(t, srv, "POST", "/api/v1/graphs/policy/environment/proposal", adminTok, "application/json", `{"model":{"provider":"analysis"}}`, nil); code != 400 {
		t.Fatalf("agent: %d", code)
	}
	proposal, err := app.Proposals.Get(context.Background(), "policy")
	if err != nil || proposal == nil || proposal.State != "failed" {
		t.Fatalf("%+v %v", proposal, err)
	}
	if _, err := app.Store.GetApproval(context.Background(), proposal.ID); err == nil {
		t.Fatal("policy violation created approval")
	}
	provider.body = `{"environment":{"image":"alpine:3.22","network":{"setup":"none"}},"rationale":"Use the cached toolchain without package downloads."}`
	if code := call(t, srv, "POST", "/api/v1/graphs/policy/environment/proposal", adminTok, "application/json", `{"model":{"provider":"analysis"}}`, nil); code != 201 {
		t.Fatalf("compliant proposal: %d", code)
	}
	compliant, err := app.Proposals.Get(context.Background(), "policy")
	if err != nil || compliant.State != "awaiting_approval" {
		t.Fatalf("%+v %v", compliant, err)
	}
	if data, err := os.ReadFile(log); err == nil && len(data) > 0 {
		t.Fatalf("Docker invoked: %s", data)
	}
}
