package taskconfig

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/storage"
)

func good() Input {
	return Input{Title: "Read", Request: "Explain architecture", Outcome: "understand", GraphID: "factory", GraphVersion: 1, NodeID: "reader"}
}
func TestValidate(t *testing.T) {
	if err := Validate(good()); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Input){func(i *Input) { i.Title = " " }, func(i *Input) { i.Request = strings.Repeat("x", 16385) }, func(i *Input) { i.Request = "\x00" }, func(i *Input) { i.Request = string([]byte{255}) }, func(i *Input) { i.Outcome = "execute-anything" }, func(i *Input) { i.GraphVersion = 0 }, func(i *Input) { i.NodeID = "" }} {
		in := good()
		change(&in)
		if !errors.Is(Validate(in), ErrInvalid) {
			t.Fatal("invalid accepted")
		}
	}
}

type broken struct {
	storage.Store
	fail   string
	writes int
}

func (b *broken) GetGraph(context.Context, string, int) (*domain.Graph, error) {
	if b.fail == "graph" {
		return nil, errors.New("graph offline")
	}
	return &domain.Graph{Metadata: domain.GraphMetadata{Version: 1}, Spec: domain.GraphSpec{Nodes: []domain.Node{{ID: "reader"}}}}, nil
}
func (b *broken) AppendEvent(context.Context, *domain.Event) error {
	if b.fail == "audit" {
		return errors.New("audit offline")
	}
	return nil
}
func (b *broken) PutSetting(context.Context, string, []byte, int64, string) (int64, error) {
	b.writes++
	return 0, errors.New("store offline")
}
func TestFailuresDoNotAppearSuccessful(t *testing.T) {
	for _, failure := range []string{"graph", "audit", "write"} {
		b := &broken{fail: failure}
		if _, err := (Manager{Store: b}).Save(t.Context(), "", good(), 0, "operator"); err == nil {
			t.Fatal("ignored failure")
		}
		if failure != "write" && b.writes != 0 {
			t.Fatal("mutation after failed dependency")
		}
	}
}
