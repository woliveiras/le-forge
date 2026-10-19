package workspace

import (
	"github.com/woliveiras/le-forge/server/internal/domain"
	"testing"
)

func TestReplicaBranchSeparatesTaskRootNamespace(t *testing.T) {
	g := &domain.Graph{Metadata: domain.GraphMetadata{ID: "fixture"}}
	if got := replicaBranch(g, "writer", 1); got != "leforge/fixture/writer-1" {
		t.Fatal(got)
	}
	g.Metadata.Labels = map[string]string{"leforge.task": "tsk_fixture"}
	if got := replicaBranch(g, "writer", 1); got != "leforge-replicas/fixture/writer-1" {
		t.Fatal(got)
	}
}
