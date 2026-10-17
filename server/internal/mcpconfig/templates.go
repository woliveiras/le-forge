package mcpconfig

import "fmt"

// TemplateRef pins public defaults. It never conveys credentials or tool grants.
type TemplateRef struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}
type Template struct {
	TemplateRef
	Name           string `json:"name"`
	Service        string `json:"service"`
	Endpoint       string `json:"endpoint"`
	Authentication string `json:"authentication"`
	Guidance       string `json:"guidance"`
	Source         string `json:"source"`
	ReviewedAt     string `json:"reviewedAt"`
	Validation     string `json:"validation"`
}

// Published versions are immutable; future revisions must use a new version.
func Templates() []Template {
	return []Template{
		{TemplateRef{"deepwiki", 1}, "DeepWiki", "generic", "https://mcp.deepwiki.com/mcp", "none", "Public repository documentation. Review discovered tools before granting agent access.", "https://docs.devin.ai/work-with-devin/deepwiki-mcp", "2026-10-17", "Previously exercised in the installed gateway; test this connection before use."},
		{TemplateRef{"github", 1}, "GitHub read-only", "github", "https://api.githubcopilot.com/mcp/readonly", "bearer", "Use a PAT restricted to selected repositories and required read permissions. OAuth is not available yet.", "https://github.com/github/github-mcp-server/blob/main/docs/remote-server.md", "2026-10-17", "Documented configuration; live vendor acceptance pending."},
		{TemplateRef{"context7", 1}, "Context7", "generic", "https://mcp.context7.com/mcp", "bearer", "Provide a Context7 API key. Discover library documentation tools and explicitly grant access.", "https://context7.com/docs/resources/all-clients", "2026-10-17", "Documented configuration; live vendor acceptance pending."},
		{TemplateRef{"hugging-face", 1}, "Hugging Face", "generic", "https://huggingface.co/mcp", "optional-bearer", "Optional Hugging Face token. Review selected Space tools and their data/compute access separately.", "https://github.com/huggingface/hf-mcp-server", "2026-10-17", "Documented configuration; live vendor acceptance pending."},
		{TemplateRef{"wandb", 1}, "W&B", "generic", "https://mcp.withwandb.com/mcp", "bearer", "Use a W&B API key with appropriate project access; grant read tools first.", "https://github.com/wandb/wandb-mcp-server", "2026-10-17", "Documented configuration; live vendor acceptance pending."},
	}
}
func validateTemplate(in Input) error {
	if in.Template == nil {
		return nil
	}
	for _, t := range Templates() {
		if *in.Template == t.TemplateRef {
			if in.Endpoint != t.Endpoint || in.Service != t.Service {
				return fmt.Errorf("%w: template defaults changed; detach the template to customize", ErrInvalid)
			}
			return nil
		}
	}
	return fmt.Errorf("%w: unknown MCP template version", ErrInvalid)
}
