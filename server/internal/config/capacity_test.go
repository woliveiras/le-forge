package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSandboxCapacityConfiguration(t *testing.T) {
	c := Default()
	c.Auth.Disabled = true
	if e := c.Validate(); e != nil {
		t.Fatal(e)
	}
	if c.Isolation.Docker.Capacity.MaxActive != 4 || c.Isolation.Docker.Capacity.DefaultMemory != "1g" {
		t.Fatal(c.Isolation.Docker.Capacity)
	}
	for _, value := range []string{"{maxActive: 0}", "{totalMemory: -1g}", "{totalMemory: 64m, defaultMemory: 128m}"} {
		c := Default()
		c.Auth.Disabled = true
		if e := yaml.Unmarshal([]byte("isolation:\n  docker:\n    capacity: "+value), c); e != nil {
			t.Fatal(e)
		}
		if e := c.Validate(); e == nil {
			t.Fatal(value)
		}
	}
}
