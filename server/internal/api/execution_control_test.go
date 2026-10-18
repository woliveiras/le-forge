package api_test

import (
	"context"
	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/executioncontrol"
	"testing"
	"time"
)

func TestExecutionTreeAPI(t *testing.T) {
	srv, a := setupApp(t, nil)
	for _, x := range []*domain.Execution{{ID: "root", GraphID: "g", RunID: "r", Status: domain.ExecRunning}, {ID: "child", ParentExecutionID: "root", GraphID: "g", RunID: "r", Status: domain.ExecAwaitingApproval}} {
		x.StartedAt = time.Now()
		if e := a.Store.PutExecution(context.Background(), x); e != nil {
			t.Fatal(e)
		}
	}
	var tree executioncontrol.Tree
	if c := call(t, srv, "GET", "/api/v1/executions/child/tree", viewerTok, "", "", &tree); c != 200 || tree.RootID != "root" || len(tree.Nodes) != 2 {
		t.Fatal(c, tree)
	}
	if c := call(t, srv, "POST", "/api/v1/executions/child/cancel", viewerTok, "application/json", `{"scope":"child","confirm":true}`, nil); c != 403 {
		t.Fatal(c)
	}
	for _, body := range []string{`{}`, `{"scope":"invalid","confirm":true}`, `{"scope":"tree","confirm":true,"extra":1}`, `{"scope":"tree","confirm":true} {}`} {
		if c := call(t, srv, "POST", "/api/v1/executions/child/cancel", operatorTok, "application/json", body, nil); c != 400 {
			t.Fatal(body, c)
		}
	}
	for _, scope := range []string{"child", "tree"} {
		var result executioncontrol.Result
		if c := call(t, srv, "POST", "/api/v1/executions/child/cancel", operatorTok, "application/json", `{"scope":"`+scope+`","confirm":true}`, &result); c != 202 || result.Request.Scope != scope {
			t.Fatal(c, result)
		}
	}
	if c := call(t, srv, "GET", "/api/v1/executions/missing/tree", viewerTok, "", "", nil); c != 404 {
		t.Fatal(c)
	}
}
