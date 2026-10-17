export function MCPServiceGuide({service}:{service:string}) {
 return <aside aria-label="OAuth service setup">
  <p>Connections belong to this LeForge installation. Use separate IDs and names, such as Personal GitHub and Work Atlassian. Each connection has its own credentials and agent grants.</p>
  {service==='github'?<>
   <p>Register a dedicated GitHub OAuth App with the callback below and your LeForge homepage. Enter its Client ID and client secret. Start with read:user and offline_access; private repository access needs a separate permission review.</p>
   <a href="https://github.com/settings/applications/new" target="_blank" rel="noreferrer">Register GitHub OAuth App</a>
   <p>Organization approval and SSO may restrict access. GitHub does not advertise standard token revocation: also revoke the grant in GitHub account settings.</p>
  </>:service==='gitlab'?<>
   <p>Use DCR when enabled on the selected instance, or manually create a user, group or instance OAuth application. Set the exact callback below, select mcp and clear Confidential. Enter the application ID; no client secret is needed for that public client. LeForge always uses PKCE S256.</p>
   <p>Pre-registration through the UI requires GitLab 19.3 or a compatible version. Ask the administrator to enable MCP for the instance or group. REST api scope and legacy personal tokens are not MCP authorization. Only publicly reachable HTTPS instances are supported.</p>
   <a href="https://docs.gitlab.com/user/model_context_protocol/mcp_server/#reuse-a-single-oauth-application" target="_blank" rel="noreferrer">GitLab MCP registration instructions</a>
  </>:['atlassian','jira','confluence','bitbucket'].includes(service)?<>
   <p>Use a client already registered with the authorization server advertised by Atlassian MCP v2. Ask your administrator or Atlassian to provision a client for the exact callback below, Authorization Code with PKCE S256, refresh tokens and the reviewed MCP scopes. A REST API 3LO application is not assumed to work.</p>
   <p>The current MCP issuer advertises CIMD and DCR; choose automatic setup to review the available mechanism. Manual setup remains available with a compatible pre-registered client. Live acceptance is still pending. Corporate callback-domain, site and IP policies may require administrator approval.</p>
   <a href="https://github.com/atlassian/atlassian-mcp-server" target="_blank" rel="noreferrer">Atlassian MCP setup and corporate requirements</a>
  </>:<p>Get the MCP endpoint, its advertised issuer, client ID and reviewed scopes from your server administrator. Register the exact callback below. REST OAuth support alone does not establish MCP compatibility.</p>}
  <p>Defaults are configuration guidance, not live-provider certification. Consent never assigns tools to agents. Check requested scopes before saving.</p>
 </aside>;
}
