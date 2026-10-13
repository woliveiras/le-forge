package rabbitq_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/woliveiras/le-forge/server/internal/queue"
	"github.com/woliveiras/le-forge/server/internal/queue/conformance"
	"github.com/woliveiras/le-forge/server/internal/queue/rabbitq"
)

// Set LEFORGE_TEST_RABBITMQ_URL (e.g. amqp://guest:guest@localhost:5672/) to run.
func TestConformance(t *testing.T) {
	url := os.Getenv("LEFORGE_TEST_RABBITMQ_URL")
	if url == "" {
		t.Skip("LEFORGE_TEST_RABBITMQ_URL not set")
	}
	conformance.RunWith(t, func(t *testing.T) queue.Broker {
		b, err := rabbitq.New(context.Background(), rabbitq.Options{URL: url})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}, conformance.Options{AckWait: time.Second})
}
