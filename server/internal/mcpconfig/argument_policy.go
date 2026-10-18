package mcpconfig

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// ArgumentRule is an exact, case-sensitive constraint on a top-level string.
// Rules for one tool are ANDed. Once configured, unlisted tools are excluded.
type ArgumentRule struct {
	Tool     string   `json:"tool"`
	Argument string   `json:"argument"`
	Values   []string `json:"values"`
}

func validateArgumentPolicy(rules []ArgumentRule) error {
	if len(rules) > 64 {
		return fmt.Errorf("%w: at most 64 argument rules", ErrInvalid)
	}
	seen := map[string]bool{}
	for _, r := range rules {
		key := r.Tool + "/" + r.Argument
		if !toolName.MatchString(r.Tool) || !toolName.MatchString(r.Argument) || seen[key] || len(r.Values) < 1 || len(r.Values) > 32 {
			return fmt.Errorf("%w: argument rules require unique tool/argument names and 1–32 values", ErrInvalid)
		}
		seen[key] = true
		values := map[string]bool{}
		for _, v := range r.Values {
			if v == "" || len(v) > 256 || strings.TrimSpace(v) != v || strings.ContainsFunc(v, unicode.IsControl) || values[v] {
				return fmt.Errorf("%w: argument values must be distinct non-empty strings of at most 256 bytes without surrounding whitespace or controls", ErrInvalid)
			}
			values[v] = true
		}
	}
	return nil
}
func argumentPolicyTool(rules []ArgumentRule, tool Tool) bool {
	if len(rules) == 0 {
		return true
	}
	var schema struct {
		Properties map[string]struct {
			Type string `json:"type"`
		} `json:"properties"`
	}
	if json.Unmarshal(tool.InputSchema, &schema) != nil {
		return false
	}
	found := false
	for _, r := range rules {
		if r.Tool == tool.Name {
			found = true
			if schema.Properties[r.Argument].Type != "string" {
				return false
			}
		}
	}
	return found
}
func argumentPolicyAllows(rules []ArgumentRule, tool Tool, args map[string]json.RawMessage) bool {
	if !argumentPolicyTool(rules, tool) {
		return false
	}
	for _, r := range rules {
		if r.Tool != tool.Name {
			continue
		}
		var value string
		if json.Unmarshal(args[r.Argument], &value) != nil {
			return false
		}
		matched := false
		for _, allowed := range r.Values {
			if value == allowed {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}
