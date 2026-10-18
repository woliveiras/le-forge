import type { MCPTemplate, MCPConnectionInput } from '../api/types';
import { gitLabOrigin } from './mcpServices';

export function templateEndpoint(template: MCPTemplate, parameters: Record<string,string> = {}): string {
  const values: Record<string,string> = {};
  for (const parameter of template.parameters ?? []) {
    const value = parameters[parameter.name] ?? parameter.default;
    if (value.length > 2048 || parameter.options && !parameter.options.includes(value)) throw new Error(`Invalid template parameter: ${parameter.label}.`);
    values[parameter.name] = value;
  }
  if (Object.keys(parameters).some(key => !(key in values))) throw new Error('Unknown template parameter.');
  if (template.id === 'github' && template.version === 2) return values.toolset === 'default' ? template.endpoint : `https://api.githubcopilot.com/mcp/x/${values.toolset}/readonly`;
  if (template.id === 'gitlab') { if (/[?#]/.test(values.instance)) throw new Error('GitLab instance must be an HTTPS origin without query or fragment.'); gitLabOrigin(values.instance); return values.instance.replace(/\/$/, '') + '/api/v4/mcp'; }
  return template.endpoint;
}

export function templateDraft(input: MCPConnectionInput, template: MCPTemplate, parameters: Record<string,string> = {}): MCPConnectionInput {
  const endpoint = templateEndpoint(template, parameters);
  const mode = input.oauth ? 'oauth' : 'bearer';
  if (template.authenticationModes?.length && !template.authenticationModes.includes(mode)) throw new Error('Choose a supported authentication mode before applying this template.');
  const issuer = endpoint.replace(/\/api\/v4\/mcp$/, '');
  const oauth = template.id === 'gitlab' && input.oauth && input.oauth.issuer !== issuer ? {...input.oauth,issuer,clientId:''} : input.oauth;
  return {...input, oauth, service:template.service, endpoint, template:{id:template.id,version:template.version}, templateParameters:parameters};
}
