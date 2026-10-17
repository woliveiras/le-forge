package mcpconfig

import (
	"errors"
	"testing"
)

func TestTemplateValidation(t *testing.T) {
	for _, template := range Templates() {
		t.Run(template.ID, func(t *testing.T) {
			in := Input{ID: "connection", Name: "Connection", Service: template.Service, Endpoint: template.Endpoint, Template: &template.TemplateRef}
			if err := validate(in); err != nil {
				t.Fatal(err)
			}
			in.Template = &TemplateRef{ID: template.ID, Version: 999}
			if !errors.Is(validate(in), ErrInvalid) {
				t.Fatal("unknown version accepted")
			}
			in.Template = &template.TemplateRef
			in.Endpoint = "https://other.example/mcp"
			if !errors.Is(validate(in), ErrInvalid) {
				t.Fatal("changed defaults accepted")
			}
			in.Template = nil
			if err := validate(in); err != nil {
				t.Fatal("custom configuration rejected", err)
			}
		})
	}
	a := Templates()
	a[0].Name = "mutated"
	if Templates()[0].Name == "mutated" {
		t.Fatal("catalogue is mutable")
	}
}
