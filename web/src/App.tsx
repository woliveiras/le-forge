import { useCallback, useEffect, useState } from 'react';
import { api, errorMessage, getToken, MOCK, onUnauthorized, setToken } from './api/client';
import type { GraphSummary } from './api/types';
import { ApprovalsView } from './components/ApprovalsView';
import { Badge, Btn, ErrorBox, Modal } from './components/common';
import { GraphList } from './components/GraphList';
import { GraphWorkspace, type GraphTab } from './components/GraphWorkspace';
import { IntegrationsView } from './components/IntegrationsView';
import { useAsync, useThrottled } from './hooks/useData';
import { EventStreamProvider, useEventStream } from './hooks/useEventStream';

/**
 * Routes live in the hash so a reload (or a shared link) returns to the same
 * place without server-side routing:
 *   #/graphs            #/approvals            #/integrations
 *   #/g/<id>/<tab>[?c=<correlation id>]
 */
type Route =
  | { view: 'graphs' }
  | { view: 'approvals' }
  | { view: 'integrations' }
  | { view: 'graph'; id: string; tab: GraphTab; correlationId?: string };

function parseRoute(hash: string): Route {
  const [path, qs] = hash.replace(/^#/, '').split('?');
  const parts = path.split('/').filter(Boolean);
  if (parts[0] === 'approvals') return { view: 'approvals' };
  if (parts[0] === 'integrations') return { view: 'integrations' };
  if (parts[0] === 'g' && parts[1]) {
    const tab = (['canvas', 'files', 'events', 'messages', 'yaml'] as GraphTab[]).includes(parts[2] as GraphTab)
      ? (parts[2] as GraphTab)
      : 'canvas';
    const c = new URLSearchParams(qs ?? '').get('c') ?? undefined;
    return { view: 'graph', id: decodeURIComponent(parts[1]), tab, correlationId: c };
  }
  return { view: 'graphs' };
}

function routeHash(r: Route): string {
  if (r.view === 'approvals') return '#/approvals';
  if (r.view === 'integrations') return '#/integrations';
  if (r.view === 'graph')
    return `#/g/${encodeURIComponent(r.id)}/${r.tab}${r.correlationId ? `?c=${encodeURIComponent(r.correlationId)}` : ''}`;
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
        <button
          type="button"
          className={`approvals-btn ${route.view === 'integrations' ? 'on' : ''}`}
          onClick={() => go({ view: 'integrations' })}
        >
          Integrations
        </button>
        <button
          type="button"
          className={`approvals-btn ${route.view === 'approvals' ? 'on' : ''}`}
          onClick={() => go({ view: 'approvals' })}
        >
          Approvals {pending > 0 && <span className="count">{pending}</span>}
        </button>
        {info.data?.grafanaUrl && (
          <a href={info.data.grafanaUrl} target="_blank" rel="noreferrer" className="small">
            Grafana ↗
          </a>
        )}
      </header>
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
        {route.view === 'approvals' && <ApprovalsView onChanged={() => void approvals.reload()} />}
        {route.view === 'integrations' && <IntegrationsView info={info.data} infoError={info.error} />}
        {route.view === 'graph' &&
          (current ? (
            <GraphWorkspace
              key={current.graph.metadata.id}
              summary={current}
              info={info.data}
              tab={route.tab}
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
