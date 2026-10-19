package localinstall

import (
	"context"
	"encoding/json"
	"fmt"
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
	stop(cancel, done)
	cancel, done = start()
	defer stop(cancel, done)
	var restored taskconfig.Draft
	if c := request("GET", "/api/v1/tasks/"+draft.ID, "", &restored); c != 200 || restored.Title != draft.Title || restored.Revision != draft.Revision {
		t.Fatal("lost draft on restart", c)
	}
	res, err := http.Get(base + "/api/v1/tasks")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatal("unauthenticated API", res.StatusCode)
	}
}
