package config

import "testing"

func TestOAuthPublicURLSetting(t *testing.T) {
	c := Default()
	if c.Server.PublicURL != "http://localhost:8080" {
		t.Fatal(c.Server.PublicURL)
	}
	t.Setenv("LEFORGE_PUBLIC_URL", "https://forge.example")
	applyEnv(c)
	if c.Server.PublicURL != "https://forge.example" {
		t.Fatal(c.Server.PublicURL)
	}
}
