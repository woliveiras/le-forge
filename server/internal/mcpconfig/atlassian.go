package mcpconfig

const atlassianMCPEndpoint = "https://mcp.atlassian.com/v2/mcp"
const atlassianFlatEndpoint = atlassianMCPEndpoint + "?tools=all"

// This is the only reviewed MCP query option. Never generalize it to arbitrary
// hosts, decoded query aliases, repeated parameters or credentials in URLs.
func flatAtlassianEndpoint(endpoint string) bool { return endpoint == atlassianFlatEndpoint }
func canonicalMCPResource(endpoint string) string {
	if flatAtlassianEndpoint(endpoint) {
		return atlassianMCPEndpoint
	}
	return endpoint
}
func oauthResource(endpoint string, metadata oauthMetadata) string {
	if metadata.Resource != "" {
		return metadata.Resource
	}
	return canonicalMCPResource(endpoint)
}
