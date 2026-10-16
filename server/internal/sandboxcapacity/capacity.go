// Package sandboxcapacity admits execution containers against an installation-wide
// budget. Durable reservations fail closed across ambiguous Docker failures/restarts.
package sandboxcapacity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/woliveiras/le-forge/server/internal/domain"
	"github.com/woliveiras/le-forge/server/internal/observability"
	"github.com/woliveiras/le-forge/server/internal/storage"
)

type Policy struct {
	MaxActive     int    `yaml:"maxActive"`
	TotalMemory   string `yaml:"totalMemory"`
	DefaultMemory string `yaml:"defaultMemory"`
}

func DefaultPolicy() Policy { return Policy{MaxActive: 4, TotalMemory: "4g", DefaultMemory: "1g"} }
func Bytes(raw string) (int64, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	multiplier := int64(1)
	if s == "" {
		return 0, fmt.Errorf("memory limit is required")
	}
	switch s[len(s)-1] {
	case 'k':
		multiplier = 1 << 10
		s = s[:len(s)-1]
	case 'm':
		multiplier = 1 << 20
		s = s[:len(s)-1]
	case 'g':
		multiplier = 1 << 30
		s = s[:len(s)-1]
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 1 || n > math.MaxInt64/multiplier {
		return 0, fmt.Errorf("invalid memory limit %q (use bytes or integer k/m/g)", raw)
	}
	return n * multiplier, nil
}
func (p Policy) Validate() error {
	total, err := Bytes(p.TotalMemory)
	if err != nil {
		return err
	}
	def, err := Bytes(p.DefaultMemory)
	if err != nil {
		return err
	}
	if p.MaxActive < 1 || def < 6<<20 || def > total {
		return fmt.Errorf("sandbox capacity requires maxActive >= 1 and 6 MiB <= defaultMemory <= totalMemory")
	}
	return nil
}

type Container struct {
	Name   string
	Memory int64
}
type Inventory func(context.Context) ([]Container, error)
type Manager struct {
	Store  storage.Store
	Policy Policy
	Scope  string
	Notify func(string, string)
}

func (m *Manager) Memory(request string) (int64, error) {
	if err := m.Policy.Validate(); err != nil {
		return 0, err
	}
	if request == "" {
		request = m.Policy.DefaultMemory
	}
	n, err := Bytes(request)
	if err != nil {
		return 0, err
	}
	total, _ := Bytes(m.Policy.TotalMemory)
	if n < 6<<20 || n > total {
		return 0, fmt.Errorf("sandbox memory request %s exceeds budget or is below Docker's 6 MiB minimum", request)
	}
	return n, nil
}

const namespace = "sandbox-capacity"

func (m *Manager) key() string { return "sandbox-capacity/" + m.Scope }
func (m *Manager) locked(ctx context.Context, fn func(context.Context) error) (err error) {
	holder := domain.NewID("capacity")
	for {
		ok, e := m.Store.AcquireLease(ctx, m.key(), holder, 90*time.Second)
		if e != nil {
			return e
		}
		if ok {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	defer func() {
		c, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		err = errors.Join(err, m.Store.ReleaseLease(c, m.key(), holder))
	}()
	bounded, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return fn(bounded)
}
func (m *Manager) load(ctx context.Context) (map[string]int64, error) {
	data, err := m.Store.KVGet(ctx, namespace, m.Scope)
	if errors.Is(err, storage.ErrNotFound) {
		return map[string]int64{}, nil
	}
	if err != nil {
		return nil, err
	}
	var slots map[string]int64
	if err := json.Unmarshal(data, &slots); err != nil {
		return nil, err
	}
	if slots == nil {
		return nil, fmt.Errorf("invalid sandbox capacity reservations")
	}
	return slots, nil
}
func (m *Manager) save(ctx context.Context, slots map[string]int64) error {
	data, err := json.Marshal(slots)
	if err != nil {
		return err
	}
	return m.Store.KVPut(ctx, namespace, m.Scope, data)
}

// Reserve waits without starting a container. Requests for the same durable
// container name are idempotent; callers must retain that name across restarts.
func (m *Manager) Reserve(ctx context.Context, name string, memory int64, inventory Inventory) (resultErr error) {
	ctx, span := observability.Start(ctx, "sandbox.admit")
	defer func() { observability.EndSpan(span, resultErr) }()
	if m.Store == nil || m.Scope == "" || name == "" || inventory == nil {
		return fmt.Errorf("sandbox capacity requires store, scope, container name and inventory")
	}

	if _, err := m.Memory(strconv.FormatInt(memory, 10)); err != nil {
		return err
	}
	total, _ := Bytes(m.Policy.TotalMemory)
	started := time.Now()
	defer func() { observability.SandboxAdmissionSeconds.Observe(time.Since(started).Seconds()) }()
	waiting := false
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		admitted := false
		err := m.locked(ctx, func(ctx context.Context) error {
			slots, err := m.load(ctx)
			if err != nil {
				return err
			}
			active, err := inventory(ctx)
			if err != nil {
				return fmt.Errorf("inspect sandbox capacity: %w", err)
			}
			usage := map[string]int64{}
			for k, v := range slots {
				if v <= 0 {
					return fmt.Errorf("invalid sandbox reservation")
				}
				usage[k] = v
			}
			for _, c := range active {
				if c.Memory <= 0 {
					return fmt.Errorf("running sandbox %s has no memory cap; stop it before admitting more work", c.Name)
				}
				if c.Memory > usage[c.Name] {
					usage[c.Name] = c.Memory
				}
			}
			if previous, ok := usage[name]; ok && previous != memory {
				return fmt.Errorf("sandbox %s has a different reserved memory limit; stop it before changing limits", name)
			}
			used := int64(0)
			count := 0
			for k, v := range usage {
				if k == name {
					continue
				}
				count++
				if v > total-used {
					used = total
					break
				}
				used += v
			}
			if count >= m.Policy.MaxActive || memory > total-used {
				if _, existing := usage[name]; existing {
					return fmt.Errorf("existing sandbox reservations exceed the configured budget; stop sandboxes before reducing limits")
				}
				return nil
			}
			slots[name] = memory
			if err := m.save(ctx, slots); err != nil {
				return err
			}
			admitted = true
			return nil
		})
		if err != nil {
			m.notify(name, "failed")
			return err
		}
		if admitted {
			m.notify(name, "admitted")
			return nil
		}
		if !waiting {
			waiting = true
			m.notify(name, "waiting")
		}
		select {
		case <-ctx.Done():
			m.notify(name, "cancelled")
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// Release is called only after Docker confirms removal. Never expire a reservation
// just because its control plane stopped: the container may still consume memory.
func (m *Manager) Release(ctx context.Context, name string) error {
	err := m.locked(ctx, func(ctx context.Context) error {
		slots, err := m.load(ctx)
		if err != nil {
			return err
		}
		delete(slots, name)
		return m.save(ctx, slots)
	})
	if err == nil {
		m.notify(name, "released")
	}
	return err
}

func (m *Manager) notify(name, state string) {
	observability.SandboxAdmissions.WithLabelValues(state).Inc()
	if m.Notify != nil {
		m.Notify(name, state)
	}
}
