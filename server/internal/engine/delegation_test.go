package engine

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseDelegation(t *testing.T) {
	for _, raw := range []string{`{}`, `null`, `{"template":"../other","objective":"work"}`, `{"template":"analysis","objective":" "}`,
		`{"template":"analysis","objective":"work","tools":["shell.exec"]}`, `{"template":"analysis","objective":"work","model":"other"}`,
		`{"template":"analysis","objective":"work"} {}`, `{"template":"analysis","objective":"` + strings.Repeat("a", 16385) + `"}`} {
		if _, _, err := parseDelegation(json.RawMessage(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	for _, objective := range []string{"Review this data", strings.Repeat("a", 4096)} {
		raw, _ := json.Marshal(map[string]string{"template": "analysis", "objective": objective})
		got, _, err := parseDelegation(raw)
		if err != nil || got != objective {
			t.Fatalf("%q %v", got, err)
		}
	}
}
