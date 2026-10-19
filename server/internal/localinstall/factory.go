package localinstall

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	"github.com/woliveiras/le-forge/server/internal/app"
	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/graph"
	"github.com/woliveiras/le-forge/server/internal/storage"
)

//go:embed software-factory.yaml
var factoryDocument []byte

const factorySeedKey = "local-install/software-factory-v1"

func factoryGraph() (*domain.Graph, error) {
	g, unknown, err := graph.Decode(factoryDocument, true)
	if err != nil {
		return nil, err
	}
	if len(unknown) > 0 {
		return nil, errors.New("native factory contains unknown fields")
	}
	graph.Normalize(g)
	report := graph.Validate(g, map[domain.RuntimeKind]bool{domain.RuntimeNative: true})
	if len(report.Errors) > 0 {
		return nil, fmt.Errorf("invalid native Software Factory: %v", report.Errors)
	}
	return g, nil
}

// Seed once, preserving existing workflows and intentional later deletion.
// A crash between graph creation and marker persistence is safe to retry.
func seedFactory(ctx context.Context, a *app.App) error {
	if _, err := a.Store.GetSetting(ctx, factorySeedKey); err == nil {
		return nil
	} else if !errors.Is(err, storage.ErrNotFound) {
		return err
	}
	if _, err := a.Store.GetGraph(ctx, "software-factory", 0); errors.Is(err, storage.ErrNotFound) {
		g, e := factoryGraph()
		if e != nil {
			return e
		}
		if _, _, e = a.Engine.SaveGraph(ctx, g, 0, "native-install"); e != nil {
			return e
		}
	} else if err != nil {
		return err
	}
	_, err := a.Store.PutSetting(ctx, factorySeedKey, []byte(`{"version":1}`), 0, "native-install")
	return err
}
