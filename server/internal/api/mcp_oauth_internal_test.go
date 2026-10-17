package api

import (
	"github.com/woliveiras/le-forge/server/internal/config"
	"testing"
)

func TestOAuthCallbackOrigin(t *testing.T) {
	for _, base := range []string{"http://localhost:8080", "http://127.0.0.1:8080", "https://forge.example"} {
		s := &Server{Deps: Deps{Config: &config.Config{Server: config.Server{PublicURL: base}}}}
		got, err := s.oauthBase()
		if err != nil || got != base {
			t.Fatal(got, err)
		}
	}
	for _, base := range []string{"", "http://forge.example", "https://user:secret@forge.example", "https://forge.example?next=evil", "https://forge.example/#x", "https://forge.example/path"} {
		s := &Server{Deps: Deps{Config: &config.Config{Server: config.Server{PublicURL: base}}}}
		if _, err := s.oauthBase(); err == nil {
			t.Fatal("invalid callback origin accepted")
		}
	}
}
