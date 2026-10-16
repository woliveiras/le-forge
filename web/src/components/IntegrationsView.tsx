import { useRef, useState } from 'react';
import { api } from '../api/client';
import type { Info, ToolInfo } from '../api/types';
import { useAsync } from '../hooks/useData';
import { Badge, Btn, ErrorBox } from './common';
import { HistoryRetentionSection } from './HistoryRetentionSection';
import { NetworkPolicySection } from './NetworkPolicySection';
import { SharedEnvironmentsSection } from './SharedEnvironmentsSection';
import { ProvidersSection, type ProvidersHandle } from './ProvidersSection';

type ProbeState = 'checking' | 'ok' | 'warning' | 'error';

function Status({
  state,
  idleLabel = 'Not tested',
  successLabel = 'Connected',
}: {
  state?: ProbeState;
  idleLabel?: string;
  successLabel?: string;
}) {
  if (!state) return <Badge>{idleLabel}</Badge>;
  if (state === 'checking') return <Badge tone="info">Testing…</Badge>;
  if (state === 'ok') return <Badge tone="ok">{successLabel}</Badge>;
  if (state === 'warning') return <Badge tone="warn">Check needed</Badge>;
  return <Badge tone="bad">Unavailable</Badge>;
}

export function IntegrationsView({ info, infoError }: { info?: Info; infoError?: string }) {
  const health = useAsync(api.health, []);
  const tools = useAsync(api.tools, []);
  const readiness = useAsync(api.ready, []);
  const providersRef = useRef<ProvidersHandle>(null);
  const [testAll, setTestAll] = useState(false);
  const role = info?.identity?.role;

  const runAllChecks = async () => {
    setTestAll(true);
    await Promise.all([health.reload(), readiness.reload(), tools.reload(), providersRef.current?.testAll()]);
    setTestAll(false);
  };

  const checks = readiness.error ? undefined : readiness.data;
  const dependenciesOk = checks && checks.storage === 'ok' && checks.queue === 'ok';
  const toolCount = tools.data?.length ?? 0;

  return (
    <div className="page integrations-page">
      <div className="page-head integrations-head">
        <div>
          <h1>Integrations</h1>
          <p className="muted">
            Configure model providers and check the services LeForge needs to run. Tests run from the LeForge server.
          </p>
        </div>
        <Btn tone="primary" onClick={() => void runAllChecks()} disabled={testAll}>
          {testAll ? 'Testing integrations…' : 'Test all'}
        </Btn>
      </div>

      <section className="integration-section">
        <div className="section-head">
          <div>
            <h2>LeForge services</h2>
            <p className="muted small">
              Basic API availability, the storage and queue readiness checks, and the sandbox runtime used by docker-isolated agents.
            </p>
          </div>
          <Btn small onClick={() => void readiness.reload()} disabled={readiness.loading}>
            {readiness.loading ? 'Checking…' : 'Check services'}
          </Btn>
        </div>
        <div className="integration-grid">
          <article className="integration-card">
            <div className="integration-card-head">
              <h3>Control API</h3>
              <Status
                state={health.data?.status === 'ok' ? 'ok' : health.error ? 'error' : undefined}
                idleLabel={health.loading ? 'Checking' : 'Not checked'}
              />
            </div>
            <p className="muted small">
              {health.error ? `Health check failed: ${health.error}` : health.data?.status === 'ok'
                ? `Responding${info ? ` · ${info.instance} · version ${info.version}` : ''}`
                : infoError ?? 'Loading server health…'}
            </p>
          </article>
          <article className="integration-card">
            <div className="integration-card-head">
              <h3>Storage</h3>
              <Status
                state={checks ? (checks.storage === 'ok' ? 'ok' : 'error') : undefined}
                idleLabel={readiness.error ? 'Could not check' : readiness.loading ? 'Checking' : 'Not checked'}
              />
            </div>
            <p className="muted small">{checks?.storage ?? readiness.error ?? info?.storage ?? 'Waiting for check'}</p>
          </article>
          <article className="integration-card">
            <div className="integration-card-head">
              <h3>Queue</h3>
              <Status
                state={checks ? (checks.queue === 'ok' ? 'ok' : 'error') : undefined}
                idleLabel={readiness.error ? 'Could not check' : readiness.loading ? 'Checking' : 'Not checked'}
              />
            </div>
            <p className="muted small">{checks?.queue ?? readiness.error ?? info?.queue ?? 'Waiting for check'}</p>
          </article>
          <article className="integration-card" aria-label="Sandbox runtime">
            <div className="integration-card-head">
              <h3>Sandbox runtime</h3>
              <Status
                state={
                  !checks?.sandbox
                    ? undefined
                    : checks.sandbox === 'ok'
                      ? 'ok'
                      : checks.sandbox === 'not configured'
                        ? 'warning'
                        : 'error'
                }
                idleLabel={readiness.error ? 'Could not check' : readiness.loading ? 'Checking' : 'Not checked'}
              />
            </div>
            <p className="muted small">
              {checks?.sandbox === 'ok'
                ? 'Docker daemon dedicated to LeForge is reachable. Agents with docker isolation run their commands there, never on the host.'
                : checks?.sandbox === 'not configured'
                  ? 'No sandbox daemon is configured. Agents with docker isolation cannot start; process isolation runs commands inside the LeForge server.'
                  : (checks?.sandbox ?? readiness.error ?? 'Waiting for check')}
            </p>
          </article>
        </div>
        {checks && !dependenciesOk && (
          <p className="integration-note tone-note">One or more dependencies did not pass the readiness check.</p>
        )}
        {readiness.error && (
          <p className="integration-note tone-note">
            Readiness endpoint failed: {readiness.error}. The configured storage/queue names above are not a connection test.
          </p>
        )}
      </section>

      <NetworkPolicySection canManage={role === 'admin'} />
      <HistoryRetentionSection canManage={role === 'admin'} />

      <SharedEnvironmentsSection canManage={role === 'admin'} />

      <ProvidersSection ref={providersRef} canEdit={role === 'admin'} canTest={role === 'admin' || role === 'operator'} />

      <section className="integration-section">
        <div className="section-head">
          <div>
            <h2>Tool catalog</h2>
            <p className="muted small">Tools currently registered in LeForge and available for agent permissions.</p>
          </div>
          <Status
            state={tools.data ? (toolCount > 0 ? 'ok' : 'warning') : tools.error ? 'error' : undefined}
            idleLabel={tools.loading ? 'Checking' : 'Not checked'}
            successLabel="Catalog loaded"
          />
        </div>
        {tools.error && <ErrorBox error={`Could not load tool catalog: ${tools.error}`} />}
        {tools.data && (
          <>
            <p className="integration-result">{toolCount} tools registered</p>
            <div className="integration-tools">
              {tools.data.map((tool: ToolInfo) => (
                <article className="integration-tool" key={tool.name} title={tool.description}>
                  <strong>{tool.name}</strong>
                  {tool.capability !== tool.name && <span className="muted small">{tool.capability}</span>}
                </article>
              ))}
            </div>
            <p className="muted small integration-footnote">
              A registered tool is not necessarily an external service connection. MCP server management is not available in this version.
            </p>
          </>
        )}
      </section>
    </div>
  );
}
