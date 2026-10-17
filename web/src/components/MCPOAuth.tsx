import { useAsync } from '../hooks/useData';
import { useState } from 'react';
import { api, errorMessage } from '../api/client';
import type { MCPConnection } from '../api/types';
import { Btn, ErrorBox } from './common';

export function MCPOAuthReturn({query,canManage,onChanged}:{query:string;canManage:boolean;onChanged:()=>Promise<void>}) {
 const params=new URLSearchParams(query);
 const state=params.get('state')??'';
 const valid=/^[A-Za-z0-9_-]{43}$/.test(state)&&params.size===1;
 const [busy,setBusy]=useState(false);const [error,setError]=useState<string>();const [result,setResult]=useState<string>();
 async function complete(){setBusy(true);setError(undefined);try{
  const connection=await api.completeMCPOAuth(state);
  setResult(connection.status==='ready'?'OAuth connected. Select tools for agents explicitly.':'OAuth authorized, but tool discovery failed. Test the connection to retry discovery.');
  window.history.replaceState(null,'',`${location.pathname}${location.search}#/integrations`);
  await onChanged();
 }catch(e){setError(errorMessage(e));}finally{setBusy(false);}}
 return <section aria-label="OAuth return"><h3>Return from OAuth provider</h3>
  {!valid?<ErrorBox error="Invalid OAuth return. Start authorization again from the connection."/>:<>
   <p>Complete the connection using the same LeForge login that started authorization.</p>
   <ErrorBox error={error}/>{result?<p role="status">{result}</p>:canManage?<Btn disabled={busy} onClick={()=>void complete()}>Complete OAuth connection</Btn>:<p>Sign in as the administrator who started this connection.</p>}
  </>}
 </section>;
}
export function MCPOAuthActions({connection,canManage,onChanged}:{connection:MCPConnection;canManage:boolean;onChanged:()=>Promise<void>}) {
 const [busy,setBusy]=useState(false);const [error,setError]=useState<string>();const [confirm,setConfirm]=useState(false);
 const oauth=connection.oauth!;
 async function start(){setBusy(true);setError(undefined);try{const result=await api.startMCPOAuth(connection.id);window.location.assign(result.url);}catch(e){setError(errorMessage(e));setBusy(false);}}
 async function revoke(){setBusy(true);setError(undefined);try{await api.revokeMCPOAuth(connection.id);setConfirm(false);await onChanged();}catch(e){setError(errorMessage(e));}finally{setBusy(false);}}
 return <section aria-label={`OAuth ${connection.name}`}>
  <p>OAuth: {oauth.status} · Issuer: {oauth.issuer}</p><p>Requested scopes: {oauth.scopes.join(', ')||'Provider defaults'} · Granted: {oauth.grantedScopes?.join(', ')||'Not authorized'}</p>
  {oauth.expiresAt&&<p>Token expires: {oauth.expiresAt}. Renewal is attempted before use when supported.</p>}
  {oauth.revocation&&<p>Local access revoked. Provider revocation: {oauth.revocation}. {oauth.revocation!=='requested'&&'Review and revoke access in the provider account settings.'}</p>}
  <ErrorBox error={error}/>
  {canManage&&<div className="btn-row"><Btn disabled={busy||!connection.enabled} onClick={()=>void start()}>Authorize {connection.name}</Btn>
   {confirm?<><p>Revoke this connection for all agents? Already dispatched calls cannot be undone.</p><Btn disabled={busy} onClick={()=>void revoke()}>Confirm revoke {connection.name}</Btn><Btn disabled={busy} onClick={()=>setConfirm(false)}>Cancel revocation</Btn></>:<Btn disabled={busy||oauth.status==='revoked'} onClick={()=>setConfirm(true)}>Revoke OAuth {connection.name}</Btn>}
  </div>}
 </section>;
}

export function MCPOAuthCallbackURL() {
 const config=useAsync(api.mcpOAuthConfig,[]);
 return <div><p>Register a client with your provider using this callback URL:</p>
  {config.data&&<code>{config.data.callbackUrl}</code>}
  {config.loading&&<p role="status">Loading callback URL…</p>}
  <ErrorBox error={config.error}/>
  {config.error&&<Btn onClick={()=>void config.reload()}>Retry callback configuration</Btn>}
 </div>;
}
