package redisq_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/woliveiras/le-forge/server/internal/queue"
	"github.com/woliveiras/le-forge/server/internal/queue/conformance"
	"github.com/woliveiras/le-forge/server/internal/queue/redisq"
)

// Set LEFORGE_TEST_REDIS_URL (e.g. redis://localhost:6379/15, Redis >= 7) to run.
func TestConformance(t *testing.T) {
	url := os.Getenv("LEFORGE_TEST_REDIS_URL")
	if url == "" {
		t.Skip("LEFORGE_TEST_REDIS_URL not set")
	}
	conformance.RunWith(t, func(t *testing.T) queue.Broker {
		b, err := redisq.New(context.Background(), url)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}, conformance.Options{AckWait: 2 * time.Second})
}
