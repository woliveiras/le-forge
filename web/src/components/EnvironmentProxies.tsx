import { useEffect, useState } from 'react';
import { api } from '../api/client';
import type { ProxyStatus } from '../api/types';
import { useAsync } from '../hooks/useData';
import { Btn, ErrorBox, fmtDateTime } from './common';

const reasons: Record<string, string> = {
  host_not_allowed: 'Host is not allowed', private_address: 'DNS resolved to a private or reserved address',
  invalid_destination: 'Invalid destination', unsupported_protocol: 'Unsupported protocol', unsupported_port: 'Unsupported port',
  capacity_exhausted: 'Proxy request capacity exhausted', upstream_failure: 'Upstream connection failed', unspecified: 'Reason not recorded',
};

export function EnvironmentProxies({ graphId }: { graphId: string }) {
  const [open, setOpen] = useState(false);
  return <section aria-label="Environment proxy diagnostics">
    <Btn small aria-expanded={open} onClick={() => setOpen(value => !value)}>{open ? 'Hide proxy diagnostics' : 'Show proxy diagnostics'}</Btn>
    {open && <ProxyDiagnostics key={graphId} graphId={graphId} />}
  </section>;
}
function ProxyCard({ proxy, historical = false }: { proxy: ProxyStatus; historical?: boolean }) {
  const blocked = proxy.requests.filter(request => request.outcome === 'denied');
  const failed = proxy.requests.filter(request => request.outcome === 'failed');
  const issues = proxy.requests.filter(request => request.outcome !== 'allowed');
  return <article className="integration-card">
    <h4>{historical ? 'Last preparation snapshot' : proxy.phase === 'preparation' ? 'Build/setup proxy' : proxy.phase === 'runtime' ? 'Runtime proxy' : 'Proxy unavailable'}</h4>
    <p><code>{proxy.name}</code></p>
    <p>{historical ? 'State when captured' : 'Container state'}: <strong>{proxy.state}</strong> · {fmtDateTime(proxy.capturedAt)}</p>
    <ErrorBox error={proxy.warning} />
    {!proxy.warning && <>
      <p>{blocked.length} blocked · {failed.length} failed · {proxy.requests.length} sampled requests</p>
      {issues.length === 0 ? <p>No blocks or failures in this sample.</p> : <ul>{issues.slice().reverse().map((request, index) => <li key={`${request.time}-${index}`}>
        {fmtDateTime(request.time)} · {request.host || 'Destination not recorded'} · {reasons[request.reason] ?? 'Reason not recorded'} ({request.outcome})
      </li>)}</ul>}
    </>}
  </article>;
}
function ProxyDiagnostics({ graphId }: { graphId: string }) {
  const status = useAsync(() => api.environmentProxies(graphId), [graphId]);
  useEffect(() => {
    if (status.loading) return;
    const timer = setTimeout(() => void status.reload(), 10000);
    return () => clearTimeout(timer);
  }, [status.loading, status.reload]);
  return <>
    <p>Process state is not a connectivity check. Samples contain at most 100 recent log lines per proxy from the last 24 hours; counts are not lifetime totals. HTTPS tunnels appear after closing.</p>
    <Btn small disabled={status.loading} onClick={() => void status.reload()}>Refresh proxy diagnostics</Btn>
    <ErrorBox error={status.error} />
    {status.error ? <p>Current proxy state could not be verified.</p> : status.loading ? <p>Reading proxy diagnostics…</p> : status.data && <>
      {status.data.truncated && <p>Only the first 32 proxies are shown.</p>}
      {status.data.proxies.length === 0 && <p>No monitored proxy containers found. Preparation proxies are removed after builds; older runtime containers need stop/start to gain monitoring labels.</p>}
      {status.data.proxies.map(proxy => <ProxyCard key={proxy.name} proxy={proxy} />)}
      {status.data.lastPreparation && <ProxyCard proxy={status.data.lastPreparation} historical />}
    </>}
    <p>The last preparation snapshot survives container cleanup until the next captured preparation or environment reset. Runtime samples disappear when their containers are removed. Direct connections blocked by network isolation do not produce proxy logs.</p>
  </>;
}
