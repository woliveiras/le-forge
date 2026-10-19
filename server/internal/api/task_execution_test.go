package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/mcpconfig"
	"github.com/woliveiras/le-forge/server/internal/mcpconfig/mcptest"
	"github.com/woliveiras/le-forge/server/internal/storage"
	"github.com/woliveiras/le-forge/server/internal/taskconfig"
	"github.com/woliveiras/le-forge/server/internal/taskrun"
	"net/http"
	"strings"
	"testing"
	"time"
)

func taskStart(t *testing.T, srvURL, id, token string, rev int64) int {
	t.Helper()
	req, _ := http.NewRequest("POST", srvURL+"/api/v1/tasks/"+id+"/start", strings.NewReader(`{"confirm":true}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("If-Match", fmt.Sprint(rev))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	return res.StatusCode
}
func TestTaskExecutionCopiesAndIdempotentStart(t *testing.T) {
	srv, a := setupApp(t, nil)
	if c := call(t, srv, "POST", "/api/v1/graphs", adminTok, "application/json", taskGraph, nil); c != 201 {
		t.Fatal(c)
	}
	body := strings.Replace(taskBody, `"nodeId":"reader"`, `"nodeId":"reader","resultNodeId":"reader"`, 1)
	var drafts []taskconfig.Draft
	for range 2 {
		var d taskconfig.Draft
		if c := call(t, srv, "POST", "/api/v1/tasks", operatorTok, "application/json", body, &d); c != 201 {
			t.Fatal(c)
		}
		var r taskrun.Review
		if c := call(t, srv, "GET", "/api/v1/tasks/"+d.ID+"/review", viewerTok, "", "", &r); c != 200 || !r.CanStart {
			t.Fatal(c, r)
		}
		if c := taskStart(t, srv.URL, d.ID, operatorTok, d.Revision); c != 403 {
			t.Fatal("operator escalated graph creation", c)
		}
		if c := taskStart(t, srv.URL, d.ID, adminTok, d.Revision); c != 202 {
			t.Fatal(c)
		}
		if c := taskStart(t, srv.URL, d.ID, adminTok, d.Revision); c != 202 {
			t.Fatal("retry", c)
		}
		drafts = append(drafts, d)
	}
	for _, d := range drafts {
		waitFor(t, 5*time.Second, "task result", func() bool {
			var v taskrun.View
			call(t, srv, "GET", "/api/v1/tasks/"+d.ID+"/progress", viewerTok, "", "", &v)
			return v.State == "succeeded" && len(v.Executions) == 1 && v.Result != nil
		})
		var page taskconfig.Page
		if c := call(t, srv, "GET", "/api/v1/tasks", viewerTok, "", "", &page); c != 200 {
			t.Fatal(c)
		}
		found := false
		for _, item := range page.Items {
			if item.ID == d.ID {
				found = item.ObservedState == "succeeded"
			}
		}
		if !found {
			t.Fatal("list omitted observed outcome")
		}
		gid := taskrun.ExecutionID(d.ID)
		for _, action := range []string{"start", "stop", "kill"} {
			if c := call(t, srv, "POST", "/api/v1/graphs/"+gid+"/"+action, adminTok, "application/json", `{}`, nil); c != 409 {
				t.Fatal(action, c)
			}
		}
		if c := call(t, srv, "POST", "/api/v1/graphs/"+gid+"/nodes/reader/chat", adminTok, "application/json", `{"text":"unrelated"}`, nil); c != 409 {
			t.Fatal(c)
		}
		if c := call(t, srv, "POST", "/api/v1/tasks/"+d.ID+"/cancel", adminTok, "application/json", `{"confirm":true}`, nil); c != 202 {
			t.Fatal(c)
		}
	}
	xs, err := a.Store.ListExecutions(t.Context(), storage.ExecutionFilter{GraphID: "task-fixture"})
	if err != nil || len(xs) > 0 {
		t.Fatal("source ran", xs, err)
	}
	g, err := a.Store.GetGraph(t.Context(), "task-fixture", 0)
	if err != nil || g.Metadata.Version != 1 {
		t.Fatal(g, err)
	}
}
func TestTaskUncertainPreparationIsNotReplayed(t *testing.T) {
	srv, a := setupApp(t, nil)
	call(t, srv, "POST", "/api/v1/graphs", adminTok, "application/json", taskGraph, nil)
	var d taskconfig.Draft
	call(t, srv, "POST", "/api/v1/tasks", operatorTok, "application/json", taskBody, &d)
	d.ExecutionGraphID = taskrun.ExecutionID(d.ID)
	m := taskconfig.Manager{Store: a.Store}
	claimed, err := m.Transition(t.Context(), &d, "starting", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if c := taskStart(t, srv.URL, d.ID, adminTok, claimed.Revision); c != 202 {
		t.Fatal(c)
	}
	if _, err = a.Store.GetGraph(t.Context(), d.ExecutionGraphID, 0); err != storage.ErrNotFound {
		t.Fatal("uncertain launch replayed", err)
	}
	// Public creation cannot occupy the reserved execution namespace.
	doc := map[string]any{"metadata": map[string]any{"id": "task-run-fake"}, "spec": map[string]any{"nodes": []domain.Node{{ID: "x", Runtime: domain.RuntimeRef{Kind: domain.RuntimeEcho}}}}}
	b, _ := json.Marshal(doc)
	if c := call(t, srv, "POST", "/api/v1/graphs", adminTok, "application/json", string(b), nil); c != 400 {
		t.Fatal(c)
	}
}

func TestTaskAdmissionRejectsUnreviewedAndStaleRequests(t *testing.T) {
	srv, _ := setupApp(t, nil)
	call(t, srv, "POST", "/api/v1/graphs", adminTok, "application/json", taskGraph, nil)
	var d taskconfig.Draft
	call(t, srv, "POST", "/api/v1/tasks", operatorTok, "application/json", taskBody, &d)
	if c := taskStart(t, srv.URL, d.ID, adminTok, d.Revision); c != 422 {
		t.Fatal("missing result binding", c)
	}
	if c := call(t, srv, "POST", "/api/v1/tasks/"+d.ID+"/start", adminTok, "application/json", `{"confirm":false}`, nil); c != 400 {
		t.Fatal(c)
	}
	if c := taskStart(t, srv.URL, d.ID, adminTok, d.Revision+1); c != 409 {
		t.Fatal(c)
	}
	if c := call(t, srv, "POST", "/api/v1/tasks/"+d.ID+"/cancel", viewerTok, "application/json", `{"confirm":true}`, nil); c != 403 {
		t.Fatal(c)
	}
	if c := call(t, srv, "POST", "/api/v1/tasks/"+d.ID+"/cancel", adminTok, "application/json", `{"confirm":true}`, nil); c != 202 {
		t.Fatal(c)
	}
	var v taskrun.View
	if c := call(t, srv, "GET", "/api/v1/tasks/"+d.ID+"/progress", viewerTok, "", "", &v); c != 200 || v.State != "cancelled" || len(v.Executions) != 0 {
		t.Fatal(c, v)
	}
}

func TestTaskChannelInputAndRetainedOutput(t *testing.T) {
	srv, a := setupApp(t, nil)
	doc := `{"metadata":{"id":"task-channel"},"spec":{"nodes":[{"id":"worker","runtime":{"kind":"echo"},"loop":{"mode":"event"}}],"channels":[{"id":"requests","to":["worker"],"types":["feature.*"]},{"id":"result","from":"worker"}]}}`
	if c := call(t, srv, "POST", "/api/v1/graphs", adminTok, "application/json", doc, nil); c != 201 {
		t.Fatal(c)
	}
	input := taskconfig.Input{Title: "Channel task", Request: "Explain", Outcome: "freeform", GraphID: "task-channel", GraphVersion: 1, NodeID: "worker", ResultNodeID: "worker", ChannelID: "requests", MessageType: "feature.request", PayloadJSON: `{"title":"structured input"}`}
	body, _ := json.Marshal(input)
	var d taskconfig.Draft
	if c := call(t, srv, "POST", "/api/v1/tasks", adminTok, "application/json", string(body), &d); c != 201 {
		t.Fatal(c)
	}
	if c := taskStart(t, srv.URL, d.ID, adminTok, d.Revision); c != 202 {
		t.Fatal(c)
	}
	waitFor(t, 5*time.Second, "channel result", func() bool {
		var v taskrun.View
		call(t, srv, "GET", "/api/v1/tasks/"+d.ID+"/progress", viewerTok, "", "", &v)
		return v.State == "succeeded" && v.Result != nil && v.Result.Summary == "echoed 1 message(s)"
	})
	if c := taskStart(t, srv.URL, d.ID, adminTok, d.Revision); c != 202 {
		t.Fatal(c)
	}
	xs, err := a.Store.ListExecutions(t.Context(), storage.ExecutionFilter{GraphID: taskrun.ExecutionID(d.ID)})
	if err != nil || len(xs) != 1 {
		t.Fatal("duplicate channel input", len(xs), err)
	}
}

func TestTaskMessageRejectionIsNotReportedAsRunningForever(t *testing.T) {
	srv, a := setupApp(t, nil)
	doc := `{"metadata":{"id":"task-held"},"spec":{"nodes":[{"id":"worker","runtime":{"kind":"echo"},"loop":{"mode":"event"}}],"channels":[{"id":"requests","to":["worker"],"approval":true}]}}`
	if c := call(t, srv, "POST", "/api/v1/graphs", adminTok, "application/json", doc, nil); c != 201 {
		t.Fatal(c)
	}
	input := taskconfig.Input{Title: "Held task", Request: "Explain", Outcome: "freeform", GraphID: "task-held", GraphVersion: 1, NodeID: "worker", ResultNodeID: "worker", ChannelID: "requests", MessageType: "feature.request", PayloadJSON: `{}`}
	body, _ := json.Marshal(input)
	var d taskconfig.Draft
	call(t, srv, "POST", "/api/v1/tasks", adminTok, "application/json", string(body), &d)
	if c := taskStart(t, srv.URL, d.ID, adminTok, d.Revision); c != 202 {
		t.Fatal(c)
	}
	approvals, err := a.Store.ListApprovals(t.Context(), storage.ApprovalFilter{GraphID: taskrun.ExecutionID(d.ID), Status: domain.ApprovalPending})
	if err != nil || len(approvals) != 1 {
		t.Fatal(approvals, err)
	}
	var v taskrun.View
	call(t, srv, "GET", "/api/v1/tasks/"+d.ID+"/progress", viewerTok, "", "", &v)
	if len(v.PendingApprovals) != 1 || v.PendingApprovals[0].ID != approvals[0].ID || v.PendingApprovals[0].GraphID != taskrun.ExecutionID(d.ID) {
		t.Fatal("missing scoped pending approval", v.PendingApprovals)
	}
	if len(v.Blockers) == 0 {
		t.Fatal("missing action needed")
	}
	if c := call(t, srv, "POST", "/api/v1/approvals/"+approvals[0].ID+"/reject", adminTok, "application/json", `{}`, nil); c != 200 {
		t.Fatal(c)
	}
	waitFor(t, 5*time.Second, "rejected task", func() bool {
		call(t, srv, "GET", "/api/v1/tasks/"+d.ID+"/progress", viewerTok, "", "", &v)
		return v.State == "failed" && len(v.Executions) == 0 && v.PendingApprovals != nil && len(v.PendingApprovals) == 0
	})
}

func TestTaskConnectionRepairPreservesUnsubmittedDraft(t *testing.T) {
	srv, a := setupApp(t, nil)
	upstream := mcptest.New()
	defer upstream.Close()
	a.MCP.Discover = func(ctx context.Context, endpoint, token string) ([]mcpconfig.Tool, error) {
		return mcpconfig.DiscoverWithClient(ctx, endpoint, token, upstream.Client())
	}
	in := mcpconfig.Input{ID: "task-repair", Name: "Task repair", Service: "generic", Endpoint: upstream.URL, Enabled: true, Credential: &mcpconfig.Credential{Value: "fixture-mcp-token"}}
	if _, err := a.MCP.Put(t.Context(), in, 0, "admin"); err != nil {
		t.Fatal(err)
	}
	connection, err := a.MCP.Refresh(t.Context(), in.ID, "admin")
	if err != nil || connection.Status != "ready" {
		t.Fatal(connection, err)
	}
	doc := `{"metadata":{"id":"task-repair"},"spec":{"nodes":[{"id":"worker","runtime":{"kind":"echo"},"loop":{"mode":"once"},"tools":["mcp.task-repair.search"]}]}}`
	if c := call(t, srv, "POST", "/api/v1/graphs", adminTok, "application/json", doc, nil); c != 201 {
		t.Fatal(c)
	}
	input := taskconfig.Input{Title: "Repair task", Request: "Preserve this request", Outcome: "freeform", GraphID: "task-repair", GraphVersion: 1, NodeID: "worker", ResultNodeID: "worker"}
	body, _ := json.Marshal(input)
	var d taskconfig.Draft
	if c := call(t, srv, "POST", "/api/v1/tasks", adminTok, "application/json", string(body), &d); c != 201 {
		t.Fatal(c)
	}
	in.Enabled = false
	in.Credential = nil
	connection, err = a.MCP.Put(t.Context(), in, connection.Revision, "admin")
	if err != nil {
		t.Fatal(err)
	}
	var review taskrun.Review
	if c := call(t, srv, "GET", "/api/v1/tasks/"+d.ID+"/review", adminTok, "", "", &review); c != 200 || review.CanStart || len(review.Blockers) == 0 {
		t.Fatal(c, review)
	}
	var retained taskconfig.Draft
	call(t, srv, "GET", "/api/v1/tasks/"+d.ID, adminTok, "", "", &retained)
	if retained.Status != "draft" || retained.Revision != d.Revision || retained.Request != input.Request || retained.ExecutionGraphID != "" {
		t.Fatal(retained)
	}
	in.Enabled = true
	if _, err = a.MCP.Put(t.Context(), in, connection.Revision, "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err = a.MCP.Refresh(t.Context(), in.ID, "admin"); err != nil {
		t.Fatal(err)
	}
	if c := call(t, srv, "GET", "/api/v1/tasks/"+d.ID+"/review", adminTok, "", "", &review); c != 200 || !review.CanStart || review.Task.Revision != d.Revision {
		t.Fatal(c, review)
	}
	xs, err := a.Store.ListExecutions(t.Context(), storage.ExecutionFilter{GraphID: taskrun.ExecutionID(d.ID)})
	if err != nil || len(xs) != 0 {
		t.Fatal("review dispatched execution", xs, err)
	}
}
