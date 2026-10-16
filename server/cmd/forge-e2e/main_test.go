//go:build e2e

package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/woliveiras/le-forge/server/internal/environment"
	"github.com/woliveiras/le-forge/server/internal/llm"
)

func TestAnalysisFixture(t *testing.T) {
	p := analysisProvider{}
	response, err := p.Chat(context.Background(), &llm.ChatRequest{Messages: []llm.ChatMessage{{Content: "package.json: ordinary project"}}})
	if err != nil {
		t.Fatal(err)
	}
	var proposal environment.Analysis
	if err := json.Unmarshal([]byte(response.Message.Content), &proposal); err != nil {
		t.Fatal(err)
	}
	if err := environment.ValidateAnalysis(&proposal); err != nil {
		t.Fatal(err)
	}
	if proposal.Environment.Image != "alpine:3.22" || proposal.Rationale == "" {
		t.Fatalf("invalid fixture: %+v", proposal)
	}
	response, err = p.Chat(context.Background(), &llm.ChatRequest{Messages: []llm.ChatMessage{{Content: "package.json: analysis-failure"}}})
	if response != nil || err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("failure fixture: %v %v", response, err)
	}
}
