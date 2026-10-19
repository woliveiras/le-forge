package localinstall

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/taskconfig"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeStartPersistRestartAndOccupiedPort(t *testing.T) {
	root, web := fixture(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err = Init(root, web, port); err != nil {
		t.Fatal(err)
	}
	// An occupied port fails without admitting background work or replacing data.
	if err = Run(t.Context(), root); err == nil {
		t.Fatal("occupied port accepted")
	}
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	// This credential was generated exclusively inside this disposable test fixture.
	b, err := os.ReadFile(filepath.Join(root, "admin.token"))
	if err != nil {
		t.Fatal(err)
	}
	token := strings.TrimSpace(string(b))
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	request := func(method, path, body string, out any) int {
		t.Helper()
		req, err := http.NewRequest(method, base+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if out != nil {
			if err = json.NewDecoder(res.Body).Decode(out); err != nil {
				t.Fatal(err)
			}
		} else {
			_, _ = io.Copy(io.Discard, res.Body)
		}
		return res.StatusCode
	}
	start := func() (context.CancelFunc, chan error) {
		t.Helper()
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- Run(ctx, root) }()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			res, err := http.Get(base + "/healthz")
			if err == nil {
				res.Body.Close()
				if res.StatusCode == 200 {
					return cancel, done
				}
			}
			select {
			case err := <-done:
				cancel()
				t.Fatal("native startup", err)
			default:
			}
			time.Sleep(20 * time.Millisecond)
		}
		cancel()
		t.Fatal("native startup timeout")
		return nil, nil
	}
	stop := func(cancel context.CancelFunc, done chan error) {
		t.Helper()
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("native shutdown timeout")
		}
	}
	cancel, done := start()
	if c := request("GET", "/", "", nil); c != 200 {
		t.Fatal(c)
	}
	if c := request("POST", "/api/v1/graphs", `{"metadata":{"id":"fixture"},"spec":{"nodes":[{"id":"worker","runtime":{"kind":"echo"}}]}}`, nil); c != 201 {
		t.Fatal(c)
	}
	var draft taskconfig.Draft
	if c := request("POST", "/api/v1/tasks", `{"title":"Native fixture","request":"Synthetic request","outcome":"freeform","graphId":"fixture","graphVersion":1,"nodeId":"worker"}`, &draft); c != 201 {
		t.Fatal(c)
	}
	queued := `{"metadata":{"id":"queued"},"spec":{"nodes":[{"id":"worker","runtime":{"kind":"echo"},"loop":{"mode":"event"}}],"channels":[{"id":"pending","to":["worker"]},{"id":"held","to":["worker"],"approval":true}]}}`
	if c := request("POST", "/api/v1/graphs", queued, nil); c != 201 {
		t.Fatal(c)
	}
	for _, channel := range []string{"pending", "held"} {
		if c := request("POST", "/api/v1/graphs/queued/channels/"+channel+"/messages", `{"type":"fixture","payload":{"test":true}}`, nil); c != 201 {
			t.Fatal(c)
		}
	}
	var held []domain.Approval
	if c := request("GET", "/api/v1/approvals?status=pending", "", &held); c != 200 || len(held) != 1 {
		t.Fatal("missing durable gate", c)
	}
	stop(cancel, done)
	archive := filepath.Join(filepath.Dir(root), "native-backup.tar.gz")
	if err = Backup(t.Context(), root, archive); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(root, root+"-original-retained"); err != nil {
		t.Fatal(err)
	}
	if err = Restore(t.Context(), archive, root); err != nil {
		t.Fatal(err)
	}
	cancel, done = start()
	defer stop(cancel, done)
	var restored taskconfig.Draft
	if c := request("GET", "/api/v1/tasks/"+draft.ID, "", &restored); c != 200 || restored.Title != draft.Title || restored.Revision != draft.Revision {
		t.Fatal("lost draft on restart", c)
	}
	var recovered []domain.Approval
	if c := request("GET", "/api/v1/approvals?status=pending", "", &recovered); c != 200 || len(recovered) != 1 || recovered[0].ID != held[0].ID {
		t.Fatal("lost approval", c)
	}
	if c := request("POST", "/api/v1/graphs/queued/start", `{}`, nil); c != 200 && c != 202 {
		t.Fatal(c)
	}
	waitExecutions := func(count int) {
		t.Helper()
		deadline := time.Now().Add(6 * time.Second)
		for time.Now().Before(deadline) {
			var xs []domain.Execution
			request("GET", "/api/v1/graphs/queued/executions", "", &xs)
			if len(xs) == count {
				settled := true
				for _, x := range xs {
					settled = settled && x.Status == domain.ExecSucceeded
				}
				if settled {
					return
				}
			}
			time.Sleep(30 * time.Millisecond)
		}
		t.Fatal("restored queue did not settle", count)
	}
	waitExecutions(1)
	if c := request("POST", "/api/v1/approvals/"+held[0].ID+"/approve", `{}`, nil); c != 200 {
		t.Fatal(c)
	}
	waitExecutions(2)
	res, err := http.Get(base + "/api/v1/tasks")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatal("unauthenticated API", res.StatusCode)
	}
}
