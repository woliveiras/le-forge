package api

import (
	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/taskconfig"
	"testing"
)

func TestInspectionTaskBinding(t *testing.T) {
	d := &taskconfig.Draft{ID: "tsk_fixture", ExecutionGraphID: "task-run-fixture", Input: taskconfig.Input{GraphID: "source"}}
	g := &domain.Graph{Metadata: domain.GraphMetadata{ID: "task-run-fixture", Labels: map[string]string{"leforge.task": "tsk_fixture", "leforge.workflow": "source"}}}
	if !validInspectionTask(g, d) {
		t.Fatal("valid binding refused")
	}
	if validInspectionTask(g, nil) {
		t.Fatal("missing task accepted")
	}
	for _, field := range []string{"execution", "task", "workflow", "graph"} {
		copy := *d
		changed := *g
		changed.Metadata.Labels = map[string]string{"leforge.task": "tsk_fixture", "leforge.workflow": "source"}
		switch field {
		case "execution":
			copy.ExecutionGraphID = "task-run-other"
		case "task":
			changed.Metadata.Labels["leforge.task"] = "tsk_other"
		case "workflow":
			changed.Metadata.Labels["leforge.workflow"] = "other"
		case "graph":
			changed.Metadata.ID = "task-run-other"
		}
		if validInspectionTask(&changed, &copy) {
			t.Fatalf("accepted mismatched %s", field)
		}
	}
}
