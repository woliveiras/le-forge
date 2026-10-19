package taskrun

import (
	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/engine"
	"github.com/woliveiras/le-forge/server/internal/taskconfig"
	"testing"
)

func TestPreparePreservesAuthorityAndSeparatesIdentity(t *testing.T) {
	source := &domain.Graph{Metadata: domain.GraphMetadata{ID: "source", Version: 2}, Spec: domain.GraphSpec{Defaults: domain.NodeDefaults{Tools: []domain.ToolGrant{{Name: "filesystem.read", Paths: []string{"docs/**"}, RequiresApproval: true}}}, Nodes: []domain.Node{{ID: "worker", Runtime: domain.RuntimeRef{Kind: domain.RuntimeEcho}, Workspace: domain.NodeWorkspace{ReadOnly: true}}}}}
	d := &taskconfig.Draft{ID: "tsk_abc", Input: taskconfig.Input{Title: "Task", GraphVersion: 2, NodeID: "worker", ResultNodeID: "worker"}}
	copy, blocks, err := Prepare(d, source)
	if err != nil || len(blocks) > 0 {
		t.Fatal(blocks, err)
	}
	if copy.Metadata.ID != "task-run-abc" || copy.Metadata.Labels["leforge.task"] != d.ID || !copy.Spec.Nodes[0].Workspace.ReadOnly || len(copy.Spec.Nodes[0].Tools) != 1 || !copy.Spec.Nodes[0].Tools[0].RequiresApproval {
		t.Fatal(copy)
	}
	copy.Spec.Nodes[0].Tools[0].Paths[0] = "other"
	if source.Metadata.ID != "source" || source.Spec.Defaults.Tools[0].Paths[0] != "docs/**" {
		t.Fatal("source mutated")
	}
}
func TestPrepareRejectsUnsupportedLaunches(t *testing.T) {
	for _, change := range []func(*domain.Graph){func(g *domain.Graph) { g.Metadata.Version = 2 }, func(g *domain.Graph) { g.Spec.Workspace.Path = "/existing" }, func(g *domain.Graph) { g.Spec.Environment = &domain.EnvironmentSpec{} }, func(g *domain.Graph) { g.Spec.Nodes[0].Disabled = true }, func(g *domain.Graph) { g.Spec.Nodes[0].Loop.Mode = domain.LoopContinuous }} {
		g := &domain.Graph{Metadata: domain.GraphMetadata{ID: "source", Version: 1}, Spec: domain.GraphSpec{Nodes: []domain.Node{{ID: "worker", Runtime: domain.RuntimeRef{Kind: domain.RuntimeEcho}}}}}
		change(g)
		_, blocks, err := Prepare(&taskconfig.Draft{ID: "tsk_abc", Input: taskconfig.Input{GraphVersion: 1, NodeID: "worker", ResultNodeID: "worker"}}, g)
		if err != nil || len(blocks) == 0 {
			t.Fatal(blocks, err)
		}
	}
}

func TestPrepareChannelBinding(t *testing.T) {
	g := &domain.Graph{Metadata: domain.GraphMetadata{ID: "source", Version: 1}, Spec: domain.GraphSpec{Nodes: []domain.Node{{ID: "worker", Runtime: domain.RuntimeRef{Kind: domain.RuntimeEcho}}}, Channels: []domain.Channel{{ID: "requests", To: []string{"worker"}, Types: []string{"feature.*"}}}}}
	d := &taskconfig.Draft{ID: "tsk_x", Input: taskconfig.Input{GraphVersion: 1, NodeID: "worker", ResultNodeID: "worker", ChannelID: "requests", MessageType: "feature.request", PayloadJSON: `{"title":"test"}`}}
	if _, b, e := Prepare(d, g); e != nil || len(b) > 0 {
		t.Fatal(b, e)
	}
	d.MessageType = "other"
	if _, b, e := Prepare(d, g); e != nil || len(b) == 0 {
		t.Fatal(b, e)
	}
	d.MessageType = "feature.request"
	g.Spec.Channels[0].From = "worker"
	if _, b, e := Prepare(d, g); e != nil || len(b) == 0 {
		t.Fatal(b, e)
	}
}

func TestWorkspaceScopesOnlyKnownPreparedReplicas(t *testing.T) {
	g := &domain.Graph{Metadata: domain.GraphMetadata{ID: "task-run-fixture"}, Spec: domain.GraphSpec{Nodes: []domain.Node{{ID: "writer", Workspace: domain.NodeWorkspace{Mode: domain.WorkspaceWorktree, Subdir: "ignored"}}, {ID: "shared", Workspace: domain.NodeWorkspace{Subdir: "src"}}}}}
	st := &engine.GraphStatus{Nodes: []engine.NodeStatus{{Replicas: []*domain.ReplicaStatus{nil, {GraphID: g.Metadata.ID, NodeID: "writer", Replica: 1, Workdir: "/fixture/writer", Branch: "task-writer"}, {GraphID: g.Metadata.ID, NodeID: "shared", Workdir: "/fixture/src"}, {GraphID: "other", NodeID: "writer", Workdir: "/private"}, {GraphID: g.Metadata.ID, NodeID: "unknown", Workdir: "/unknown"}, {GraphID: g.Metadata.ID, NodeID: "writer"}}}}}
	scopes := workspaceScopes(g, st)
	if len(scopes) != 2 || scopes[0].InspectionNode != "writer" || scopes[0].Replica != 1 || scopes[0].Subdir != "" || scopes[1].InspectionNode != "" || scopes[1].Subdir != "src" {
		t.Fatalf("unexpected scopes: %+v", scopes)
	}
}
