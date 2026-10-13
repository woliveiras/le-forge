package sqlstore_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woliveiras/le-forge/server/internal/storage"
	"github.com/woliveiras/le-forge/server/internal/storage/conformance"
	"github.com/woliveiras/le-forge/server/internal/storage/sqlstore"
)

func TestSQLiteConformance(t *testing.T) {
	conformance.Run(t, func(t *testing.T) storage.Store {
		s, err := sqlstore.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "leforge.db"))
		if err != nil {
			t.Fatal(err)
		}
		return s
	})
}

// TestPostgresConformance runs against a live server when
// LEFORGE_TEST_POSTGRES_DSN is set. Each subtest gets a fresh schema.
func TestPostgresConformance(t *testing.T) {
	dsn := os.Getenv("LEFORGE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("LEFORGE_TEST_POSTGRES_DSN not set")
	}
	n := 0
	conformance.Run(t, func(t *testing.T) storage.Store {
		n++
		schema := "conf_" + strings.ToLower(strings.NewReplacer("/", "_", "-", "_").Replace(t.Name()))
		admin, err := sqlstore.OpenPostgres(context.Background(), dsn)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := admin.DB().Exec(`DROP SCHEMA IF EXISTS ` + schema + ` CASCADE; CREATE SCHEMA ` + schema); err != nil {
			t.Fatal(err)
		}
		admin.Close()
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		s, err := sqlstore.OpenPostgres(context.Background(), dsn+sep+"search_path="+schema)
		if err != nil {
			t.Fatal(err)
		}
		return s
	})
}

func TestSQLiteDSNMemory(t *testing.T) {
	s, err := sqlstore.OpenSQLite(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
}
