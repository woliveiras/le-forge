import { useState } from 'react';
import { api, errorMessage } from '../api/client';
import type { MCPConnection, MCPConnectionInput } from '../api/types';
import { useAsync } from '../hooks/useData';
import { Badge, Btn, ErrorBox, JsonView, fmtDateTime } from './common';
const empty: MCPConnectionInput = {id:'',name:'',service:'generic',endpoint:'',enabled:true};
export function MCPSection({role}:{role?:string}) {
  const list=useAsync(api.mcpConnections,[]);
  const [input,setInput]=useState<MCPConnectionInput>(empty);
  const [revision,setRevision]=useState<number>();
  const [token,setToken]=useState('');
  const [clear,setClear]=useState(false);
  const [busy,setBusy]=useState(false);
  const [error,setError]=useState<string>();
  const admin=role==='admin';const operator=admin||role==='operator';
  const edit=(c?:MCPConnection)=>{setInput(c?{id:c.id,name:c.name,service:c.service,endpoint:c.endpoint,enabled:c.enabled}:empty);setRevision(c?.revision);setToken('');setClear(false);setError(undefined);};
  async function save(){setBusy(true);setError(undefined);try{await api.saveMCP({...input,credential:clear?{clear:true}:token?{value:token}:undefined},revision);edit();await list.reload();}catch(e){setError(errorMessage(e));}finally{setBusy(false);}}
  async function discover(id:string,action:'test'|'tools/refresh'){setBusy(true);setError(undefined);try{await api.refreshMCP(id,action);await list.reload();}catch(e){setError(errorMessage(e));}finally{setBusy(false);}}
  return <section className="integration-section" aria-label="MCP connections">
    <h2>MCP connections</h2>
    <p>Register remote HTTPS Streamable HTTP servers and discover their tools. Only public network destinations are supported. Credentials stay encrypted in LeForge.</p>
    <p>Assign tools to agents in the graph editor to allow calls through the gateway. OAuth and service-specific setup are not available yet.</p>
    <Btn onClick={()=>void list.reload()} disabled={list.loading}>Reload MCP connections</Btn>
    <ErrorBox error={list.error}/><ErrorBox error={error}/>
    {list.loading&&<p role="status">Loading connections…</p>}
    {!list.loading&&!list.error&&!list.data?.length&&<p>No MCP connections configured.</p>}
    {admin&&<form className="form" onSubmit={e=>{e.preventDefault();void save();}}>
      <h3>{revision?'Edit connection':'New connection'}</h3>
      <label>Connection ID<input required pattern="[a-z][a-z0-9_-]{0,63}" value={input.id} disabled={!!revision||busy} onChange={e=>setInput({...input,id:e.target.value})}/></label>
      <label>Connection name<input required maxLength={120} value={input.name} onChange={e=>setInput({...input,name:e.target.value})}/></label>
      <label>Service<select value={input.service} onChange={e=>setInput({...input,service:e.target.value})}>{['generic','github','bitbucket','jira','confluence'].map(s=><option key={s}>{s}</option>)}</select></label>
      <label>MCP endpoint<input type="url" required placeholder="https://mcp.example.com/mcp" value={input.endpoint} onChange={e=>setInput({...input,endpoint:e.target.value})}/></label>
      <label>Bearer token<input type="password" autoComplete="new-password" value={token} disabled={clear} onChange={e=>setToken(e.target.value)}/></label>
      <p className="muted small">Leave blank to keep the existing credential. Changing the endpoint requires replacing or clearing its credential.</p>
      <label><input type="checkbox" checked={clear} onChange={e=>{setClear(e.target.checked);setToken('');}}/>Clear stored credential</label>
      <label><input type="checkbox" checked={input.enabled} onChange={e=>setInput({...input,enabled:e.target.checked})}/>Connection enabled</label>
      <div className="btn-row"><Btn type="submit" disabled={busy} tone="primary">Save MCP connection</Btn>{revision&&<Btn disabled={busy} onClick={()=>edit()}>Cancel connection edit</Btn>}</div>
    </form>}
    <div className="cards">{(list.data??[]).map(c=><article className="card" key={c.id} aria-label={`MCP ${c.name}`}>
      <h3>{c.name} <Badge>{c.status}</Badge></h3><p>{c.service} · {c.endpoint}</p>
      <p>{c.credentialConfigured?'Credential configured':'No credential'} · revision {c.revision} · catalogue {c.toolsRevision}</p>
      {c.lastTest&&<p>Last discovery: {fmtDateTime(c.lastTest)}</p>}<ErrorBox error={c.error}/>
      <div className="btn-row">{admin&&<Btn disabled={busy} onClick={()=>edit(c)}>Edit {c.name}</Btn>}{operator&&<><Btn disabled={busy||!c.enabled} onClick={()=>void discover(c.id,'test')}>Test {c.name}</Btn><Btn disabled={busy||!c.enabled} onClick={()=>void discover(c.id,'tools/refresh')}>Refresh tools for {c.name}</Btn></>}</div>
      <ul>{c.tools.map(t=><li key={t.name}><code>mcp.{c.id}.{t.name}</code><p>{t.description}</p><JsonView label="Input schema" value={t.inputSchema}/></li>)}</ul>
      {c.status==='ready'&&!c.tools.length&&<p>The server returned no tools.</p>}
    </article>)}</div>
  </section>;
}
