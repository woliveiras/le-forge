import { runSection, type RunSection } from './components/runPresentation';
import { agentSection, type AgentSection } from './components/agentSettingsModel';
import { useCallback, useEffect, useState } from 'react';
import { api, errorMessage, getToken, MOCK, onUnauthorized, setToken } from './api/client';
import type { GraphSummary } from './api/types';
import { ApprovalsView } from './components/ApprovalsView';
import { Badge, Btn, ErrorBox, Modal } from './components/common';
import { GraphList } from './components/GraphList';
import { GraphWorkspace, type GraphTab } from './components/GraphWorkspace';
import { AdministrationPage, administrationSections, ConnectionsPage, ModelsPage, type AdministrationSection } from './components/InstallationPages';
import { connectionTabs, type ConnectionTab } from './components/mcpNavigation';
import { SkillsView } from './components/SkillsView';
import { useAsync, useThrottled } from './hooks/useData';
import { EventStreamProvider, useEventStream } from './hooks/useEventStream';

/**
 * Routes live in the hash so a reload (or a shared link) returns to the same
 * place without server-side routing:
 *   #/graphs  #/approvals  #/connections  #/models  #/administration/<section>
 * Legacy #/integrations links remain MCP aliases, including OAuth returns.
 *   #/g/<id>/<tab>[?c=<correlation id>]
 */
type Route =
  | { view: 'graphs' }
  | { view: 'approvals' }
  | { view: 'connections'; connectionID?: string; connectionTab?: ConnectionTab; create?: boolean; history?: boolean; connectQuery?: string; oauthQuery?: string }
  | { view: 'models' }
  | { view: 'administration'; section: AdministrationSection }
  | { view: 'skills' }
  | { view: 'graph'; id: string; tab: GraphTab; agentId?: string; agentSection?: AgentSection; executionId?: string; runSection?: RunSection; correlationId?: string };

export function parseRoute(hash: string): Route {
  const [path, qs] = hash.replace(/^#/, '').split('?');
  const parts = path.split('/').filter(Boolean);
  if (parts[0] === 'approvals') return { view: 'approvals' };
  if (parts[0] === 'skills') return { view: 'skills' };
  if (parts[0] === 'models') return { view: 'models' };
  if (parts[0] === 'administration') return { view: 'administration', section: administrationSections.includes(parts[1] as AdministrationSection) ? parts[1] as AdministrationSection : 'health' };
  if (parts[0] === 'connections' && parts[1] === 'c') return { view: 'connections', connectionID: parts[2] ?? '', connectionTab: connectionTabs.includes(parts[3] as ConnectionTab) ? parts[3] as ConnectionTab : 'overview' };
  if (parts[0] === 'connections' && parts[1] === 'new') return { view: 'connections', create: true };
  if (parts[0] === 'connections' && parts[1] === 'history') return { view: 'connections', history: true };
  if (parts[0] === 'integrations' || parts[0] === 'connections') return { view: 'connections', connectQuery: parts[1] === 'connect' ? (hash.includes('?') ? hash.slice(hash.indexOf('?') + 1) : '') : undefined, oauthQuery: parts[1] === 'oauth' ? (qs ?? '') : undefined };
  if (parts[0] === 'g' && parts[1]) {
    const tab = (['canvas', 'agent', 'runs', 'activity', 'files', 'events', 'messages', 'yaml'] as GraphTab[]).includes(parts[2] as GraphTab)
      ? (parts[2] as GraphTab)
      : 'canvas';
    const c = new URLSearchParams(qs ?? '').get('c') ?? undefined;
    return { view: 'graph', id: decodeURIComponent(parts[1]), tab, correlationId: c, ...(tab === 'runs' ? {executionId:new URLSearchParams(qs ?? '').get('execution') ?? undefined, runSection:runSection(new URLSearchParams(qs ?? '').get('section'))} : {}), ...(tab === 'agent' ? {agentId: new URLSearchParams(qs ?? '').get('agent') ?? '', agentSection: agentSection(new URLSearchParams(qs ?? '').get('section'))} : {}) };
  }
  return { view: 'graphs' };
}

export function routeHash(r: Route): string {
  if (r.view === 'approvals') return '#/approvals';
  if (r.view === 'skills') return '#/skills';
  if (r.view === 'connections') {
    if (r.connectionID !== undefined) return `#/connections/c/${encodeURIComponent(r.connectionID)}/${r.connectionTab ?? 'overview'}`;
    if (r.create) return '#/connections/new';
    if (r.history) return '#/connections/history';
    if (r.connectQuery !== undefined) return `#/connections/connect?${r.connectQuery}`;
    if (r.oauthQuery !== undefined) return `#/connections/oauth?${r.oauthQuery}`;
    return '#/connections';
  }
  if (r.view === 'models') return '#/models';
  if (r.view === 'administration') return `#/administration/${r.section}`;
  if (r.view === 'graph')
    return `#/g/${encodeURIComponent(r.id)}/${r.tab}${r.tab === 'runs' ? (r.executionId ? `?execution=${encodeURIComponent(r.executionId)}&section=${r.runSection ?? 'result'}` : '') : r.tab === 'agent' ? `?agent=${encodeURIComponent(r.agentId ?? '')}&section=${r.agentSection ?? 'general'}` : r.correlationId ? `?c=${encodeURIComponent(r.correlationId)}` : ''}`;
  return '#/graphs';
}

export function App() {
  const [token, setTok] = useState(getToken());
  const [needLogin, setNeedLogin] = useState(false);
  useEffect(() => onUnauthorized(() => setNeedLogin(true)), []);

  return (
    <EventStreamProvider token={token}>
      <Shell />
      {needLogin && (
        <LoginDialog
          onDone={(t) => {
            setToken(t);
            setTok(t);
            setNeedLogin(false);
            location.reload();
          }}
        />
      )}
    </EventStreamProvider>
  );
}

function Shell() {
  const [route, setRoute] = useState<Route>(() => parseRoute(location.hash));
  useEffect(() => {
    const f = () => setRoute(parseRoute(location.hash));
    window.addEventListener('hashchange', f);
    return () => window.removeEventListener('hashchange', f);
  }, []);
  const go = useCallback((r: Route) => {
    location.hash = routeHash(r);
  }, []);

  const info = useAsync(() => api.info(), []);
  const graphs = useAsync(() => api.listGraphs(), []);
  const approvals = useAsync(() => api.approvals({ status: 'pending' }), []);
  const reloadGraphs = useThrottled(() => void graphs.reload(), 500);
  const reloadApprovals = useThrottled(() => void approvals.reload(), 500);
  const { connected } = useEventStream({ prefixes: ['graph.', 'approval.'] }, (ev) =>
    ev.type.startsWith('graph.') ? reloadGraphs() : reloadApprovals(),
  );

  const list = graphs.data ?? [];
  const current = route.view === 'graph' ? list.find((g) => g.graph.metadata.id === route.id) : undefined;
  const pending = approvals.data?.length ?? 0;

  const onSaved = (s: GraphSummary) => {
    graphs.setData(list.map((g) => (g.graph.metadata.id === s.graph.metadata.id ? s : g)));
  };

  return (
    <div className="app">
      <header className="topbar">
        <button type="button" className="brand" onClick={() => go({ view: 'graphs' })}>
          <img src="/favicon.svg?v=2" alt="" width={22} height={22} /> LeForge
        </button>
        <nav className="graph-tabs" aria-label="Graphs">
          {list.map((g) => (
            <button
              key={g.graph.metadata.id}
              type="button"
              className={route.view === 'graph' && route.id === g.graph.metadata.id ? 'on' : ''}
              onClick={() => go({ view: 'graph', id: g.graph.metadata.id, tab: route.view === 'graph' ? route.tab : 'canvas' })}
              title={g.graph.metadata.description}
            >
              {g.record.state === 'running' && <span className="run-dot" aria-label="running" />}
              {g.graph.metadata.name || g.graph.metadata.id}
            </button>
          ))}
        </nav>
        <span className="spacer" />
        {info.data && (
          <span className="infra" title={`instance ${info.data.instance} · version ${info.data.version}`}>
            <span className="infra-meta">{info.data.queue} · {info.data.storage}</span>
            {MOCK && <Badge tone="warn">mock</Badge>}
          </span>
        )}
        <span className={`conn ${connected ? 'on' : ''}`} title="Event stream">
          {connected ? '● live' : '○ offline'}
        </span>
        {info.data?.grafanaUrl && (
          <a href={info.data.grafanaUrl} target="_blank" rel="noreferrer" className="small">
            Grafana ↗
          </a>
        )}
      </header>
      <div className="workspace-shell">
        <nav className="primary-navigation" aria-label="Main navigation">
          <button type="button" className={`approvals-btn ${route.view === 'graphs' || route.view === 'graph' ? 'on' : ''}`} onClick={() => go({ view: 'graphs' })}>Graphs</button>
          <button
            type="button"
            className={`approvals-btn ${route.view === 'connections' ? 'on' : ''}`} aria-current={route.view === 'connections' ? 'page' : undefined}
            onClick={() => go({ view: 'connections' })}
          >
            Connections
          </button>
          <button type="button" className={`approvals-btn ${route.view === 'models' ? 'on' : ''}`} aria-current={route.view === 'models' ? 'page' : undefined} onClick={() => go({ view: 'models' })}>Models</button>
          <button type="button" className={`approvals-btn ${route.view === 'administration' ? 'on' : ''}`} aria-current={route.view === 'administration' ? 'page' : undefined} onClick={() => go({ view: 'administration', section: 'health' })}>Administration</button>
          <button
            type="button"
            className={`approvals-btn ${route.view === 'approvals' ? 'on' : ''}`} aria-current={route.view === 'approvals' ? 'page' : undefined}
            onClick={() => go({ view: 'approvals' })}
          >
            Approvals {pending > 0 && <span className="count">{pending}</span>}
          </button>
          <button type="button" className={`approvals-btn ${route.view === 'skills' ? 'on' : ''}`} aria-current={route.view === 'skills' ? 'page' : undefined} onClick={() => go({ view: 'skills' })}>
            Skills & steering
          </button>
        </nav>
        <main>
          <ErrorBox error={info.error && !graphs.error ? `info: ${info.error}` : undefined} />
          {route.view === 'graphs' && (
            <GraphList
              graphs={list}
              error={graphs.error}
              onOpen={(id) => go({ view: 'graph', id, tab: 'canvas' })}
              onChanged={() => void graphs.reload()}
            />
          )}
          {route.view === 'skills' && <SkillsView role={info.data?.identity?.role} onChanged={() => void graphs.reload()} />}
          {route.view === 'approvals' && <ApprovalsView onChanged={() => void approvals.reload()} />}
          {route.view === 'connections' && <ConnectionsPage info={info.data} connectionID={route.connectionID} connectionTab={route.connectionTab} create={route.create} history={route.history} connectQuery={route.connectQuery} oauthQuery={route.oauthQuery} />}
          {route.view === 'models' && <ModelsPage info={info.data} />}
          {route.view === 'administration' && <AdministrationPage info={info.data} infoError={info.error} section={route.section} />}
          {route.view === 'graph' &&
            (current ? (
              <GraphWorkspace
                key={current.graph.metadata.id}
                summary={current}
                info={info.data}
                tab={route.tab}
                executionId={route.executionId}
                runSection={route.runSection}
                agentId={route.agentId}
                agentSection={route.agentSection}
                onAgent={(agentId, agentSection) => go({...route, tab: 'agent', agentId, agentSection})}
                onTab={(tab) => go({ ...route, tab })}
                onSaved={onSaved}
                correlationId={route.correlationId}
                onCorrelation={(c) => go({ ...route, tab: 'messages', correlationId: c })}
              />
            ) : graphs.loading ? (
              <p className="muted page">Loading…</p>
            ) : (
              <ErrorBox error={graphs.error ?? `Graph “${route.id}” not found.`} />
            ))}
        </main>
      </div>
    </div>
  );
}

function LoginDialog({ onDone }: { onDone: (token: string) => void }) {
  const [t, setT] = useState(getToken());
  const [error, setError] = useState<string>();
  const submit = async () => {
    setToken(t.trim());
    try {
      await api.info();
      onDone(t.trim());
    } catch (e) {
      setError(errorMessage(e));
    }
  };
  return (
    <Modal title="Sign in" onClose={() => undefined}>
      <form
        className="form"
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
      >
        <p className="muted small">
          The API needs a bearer token configured by the server administrator.
        </p>
        <label>
          API token
          <input type="password" autoFocus value={t} onChange={(e) => setT(e.target.value)} autoComplete="current-password" />
        </label>
        <ErrorBox error={error} />
        <Btn tone="primary" type="submit" disabled={!t.trim()}>
          Continue
        </Btn>
      </form>
    </Modal>
  );
}
