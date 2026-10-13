import { useEffect, useState } from 'react';
import { ApiError, api, errorMessage } from '../api/client';
import type { Channel, Graph, GraphAction, GraphNode, GraphSummary, Info } from '../api/types';
import { useGraphStatus } from '../hooks/useData';
import { GraphCanvas, type Selection } from './canvas/GraphCanvas';
import { ChannelPanel } from './ChannelPanel';
import { Badge, Btn, ErrorBox, Icon } from './common';
import { EventsView } from './EventsView';
import { FilesView } from './FilesView';
import { MessagesExplorer } from './MessagesExplorer';
import { NodePanel } from './NodePanel';
import { YamlEditor } from './YamlEditor';

export type GraphTab = 'canvas' | 'files' | 'events' | 'messages' | 'yaml';

export function GraphWorkspace({
  summary,
  info,
  tab,
  onTab,
  onSaved,
  correlationId,
  onCorrelation,
}: {
  summary: GraphSummary;
  info?: Info;
  tab: GraphTab;
  onTab: (t: GraphTab) => void;
  onSaved: (s: GraphSummary) => void;
  correlationId?: string;
  onCorrelation: (c: string | undefined) => void;
}) {
  const gid = summary.graph.metadata.id;
  const { status, error: statusError, setStatus } = useGraphStatus(gid);
  const [draftGraph, setDraftGraph] = useState<Graph>(summary.graph);
  const [draftDirty, setDraftDirty] = useState(false);
  // Deep link: #/g/<id>/canvas?sel=node:<id> or ?sel=channel:<id>
  const [selection, setSelection] = useState<Selection>(() => {
    const sel = new URLSearchParams(location.hash.split('?')[1] ?? '').get('sel');
    const [kind, id] = sel?.split(':') ?? [];
    return (kind === 'node' || kind === 'channel') && id ? { kind, id } : null;
  });
  const [error, setError] = useState<string>();
  const [saveConflict, setSaveConflict] = useState(false);
  const [panelOpen, setPanelOpen] = useState(true);
  const currentSummary = draftDirty ? { ...summary, graph: draftGraph } : summary;

  useEffect(() => {
    if (!draftDirty) setDraftGraph(summary.graph);
  }, [summary.graph, draftDirty]);

  const changeGraph = (update: (graph: Graph) => Graph) => {
    setDraftGraph((current) => update(current));
    setDraftDirty(true);
    setError(undefined);
  };

  const saveGraph = async () => {
    try {
      const validation = await api.validateGraph(draftGraph);
      if (!validation.valid) {
        setError((validation.errors ?? []).map((item) => `${item.path}: ${item.message}`).join('\n') || 'Graph is invalid.');
        return;
      }
      const saved = await api.updateGraph(gid, draftGraph, summary.record.version);
      setDraftGraph(saved.graph);
      setDraftDirty(false);
      setSaveConflict(false);
      onSaved(saved);
      setError(undefined);
    } catch (e) {
      setError(errorMessage(e));
      setSaveConflict(e instanceof ApiError && e.status === 409);
    }
  };

  const addNode = (position: { x: number; y: number }, positions: Record<string, { x: number; y: number }>) => {
    changeGraph((graph) => {
      const ids = new Set(graph.spec.nodes.map((node) => node.id));
      let index = graph.spec.nodes.length + 1;
      while (ids.has(`agent-${index}`)) index += 1;
      const id = `agent-${index}`;
      const node: GraphNode = {
        id,
        name: `Agent ${index}`,
        runtime: { kind: 'native' },
        prompt: "Describe this agent's task.",
        loop: { mode: 'event' },
        position,
      };
      return {
        ...graph,
        spec: {
          ...graph.spec,
          nodes: [
            ...graph.spec.nodes.map((existing) =>
              positions[existing.id] ? { ...existing, position: positions[existing.id] } : existing,
            ),
            node,
          ],
        },
      };
    });
  };

  const removeChannelsForNodes = (channels: Channel[] | undefined, removed: Set<string>) =>
    (channels ?? []).flatMap((channel) => {
      const to = (channel.to ?? []).filter((id) => !removed.has(id));
      if (channel.from && removed.has(channel.from)) return [];
      if ((channel.to ?? []).length > 0 && to.length === 0) return [];
      return [{ ...channel, to }];
    });

  const removeSelectedNode = () => {
    if (selection?.kind !== 'node') return;
    const node = draftGraph.spec.nodes.find((item) => item.id === selection.id);
    if (!node || !confirm(`Remove agent "${node.name || node.id}" and its connected channels?`)) return;
    const removed = new Set([node.id]);
    changeGraph((graph) => ({
      ...graph,
      spec: {
        ...graph.spec,
        nodes: graph.spec.nodes.filter((item) => item.id !== node.id),
        channels: removeChannelsForNodes(graph.spec.channels, removed),
      },
    }));
    setSelection(null);
  };

  const act = async (a: GraphAction) => {
    if (a === 'kill' && !confirm('Kill every turn in flight? Inputs are redelivered; files may be left mid-edit.')) return;
    try {
      setStatus(await api.graphAction(gid, a));
      setError(undefined);
    } catch (e) {
      setError(errorMessage(e));
    }
  };

  const state = status?.state ?? summary.record.state;
  const running = state === 'running' || state === 'paused' || state === 'stopping';

  return (
    <div className="workspace">
      <div className="toolbar">
        <div className="tabs" role="tablist">
          {(['canvas', 'files', 'events', 'messages', 'yaml'] as GraphTab[]).map((t) => (
            <button
              key={t}
              type="button"
              role="tab"
              aria-selected={tab === t}
              className={tab === t ? 'on' : ''}
              onClick={() => onTab(t)}
            >
              {t === 'canvas'
                ? 'Graph'
                : t === 'files'
                  ? 'Files'
                  : t === 'yaml'
                    ? 'Definition'
                    : t === 'messages'
                      ? 'Messages'
                      : 'Events'}
            </button>
          ))}
        </div>
        <div className="toolbar-actions">
          {tab === 'canvas' && (
            <>
              {selection?.kind === 'node' && (
                <Btn small tone="danger" onClick={removeSelectedNode}>
                  Remove agent
                </Btn>
              )}
              {draftDirty && <Badge tone="warn">unsaved graph changes</Badge>}
              {saveConflict && (
                <Btn
                  small
                  tone="danger"
                  onClick={async () => {
                    if (!confirm('Reload the latest saved graph and discard your local edits?')) return;
                    try {
                      const latest = await api.getGraph(gid);
                      setDraftGraph(latest.graph);
                      setDraftDirty(false);
                      setSaveConflict(false);
                      setError(undefined);
                      onSaved(latest);
                    } catch (cause) {
                      setError(errorMessage(cause));
                    }
                  }}
                >
                  Reload latest
                </Btn>
              )}
              <Btn small onClick={() => void saveGraph()} disabled={!draftDirty} tone="primary">
                Save graph
              </Btn>
              <Btn
                small
                tone="ghost"
                disabled={!draftDirty}
                onClick={() => {
                  setDraftGraph(summary.graph);
                  setDraftDirty(false);
                  setSaveConflict(false);
                  setError(undefined);
                }}
              >
                Discard
              </Btn>
            </>
          )}
          <div className="toolbar-run-controls">
            <Badge tone={state === 'running' ? 'ok' : state === 'failed' ? 'bad' : state === 'paused' ? 'warn' : undefined}>
              {state}
            </Badge>
            {status && status.desiredState !== status.state && <span className="muted small">→ {status.desiredState}</span>}
            <span className="muted small">v{summary.record.version}</span>
            {(status?.approvalsPending ?? 0) > 0 && <Badge tone="warn">{status?.approvalsPending} approvals</Badge>}
            <Btn small tone="primary" onClick={() => act('start')} disabled={state === 'running'} title="Run all">
              <Icon name="play" /> Run all
            </Btn>
            <Btn small onClick={() => act('stop')} disabled={!running} title="Stop all after current turns">
              <Icon name="stop" /> Stop all
            </Btn>
            {state === 'paused' ? (
              <Btn small onClick={() => act('resume')}>
                Resume
              </Btn>
            ) : (
              <Btn small onClick={() => act('pause')} disabled={state !== 'running'}>
                <Icon name="pause" /> Pause
              </Btn>
            )}
            <Btn
              small
              tone="danger"
              onClick={() => act('kill')}
              disabled={!running}
              title="Kill all turns in flight"
              aria-label="Kill"
            >
              <Icon name="bolt" />
            </Btn>
          </div>
        </div>
      </div>
      <ErrorBox error={error ?? statusError} />
      {tab === 'canvas' && (
        <div className={`floor ${selection && panelOpen ? 'with-panel' : ''}`}>
          <GraphCanvas
            graph={draftGraph}
            status={status}
            selection={selection}
            editable
            onAddNode={addNode}
            onConnect={(connection) => {
              if (!connection.source || !connection.target) return;
              changeGraph((graph) => {
                const existing = new Set((graph.spec.channels ?? []).map((channel) => channel.id));
                let index = (graph.spec.channels?.length ?? 0) + 1;
                while (existing.has(`channel-${index}`)) index += 1;
                const channel: Channel = {
                  id: `channel-${index}`,
                  type: 'queue',
                  from: connection.source,
                  to: [connection.target],
                };
                return { ...graph, spec: { ...graph.spec, channels: [...(graph.spec.channels ?? []), channel] } };
              });
            }}
            onNodePositionChange={(positions) =>
              changeGraph((graph) => ({
                ...graph,
                spec: {
                  ...graph.spec,
                  nodes: graph.spec.nodes.map((node) =>
                    positions[node.id] ? { ...node, position: positions[node.id] } : node,
                  ),
                },
              }))
            }
            onNodesDelete={(nodes) => {
              const removed = new Set(nodes.map((node) => node.id));
              changeGraph((graph) => ({
                ...graph,
                spec: {
                  ...graph.spec,
                  nodes: graph.spec.nodes.filter((node) => !removed.has(node.id)),
                  channels: removeChannelsForNodes(graph.spec.channels, removed),
                },
              }));
              if (selection?.kind === 'node' && removed.has(selection.id)) setSelection(null);
            }}
            onEdgesDelete={(edges) => {
              const removedTargets = new Map<string, Set<string>>();
              for (const edge of edges) {
                const data = edge.data as { channel?: Channel; target?: string } | undefined;
                const channelId = data?.channel?.id;
                if (!channelId) continue;
                const targets = removedTargets.get(channelId) ?? new Set<string>();
                if (data.target) targets.add(data.target);
                removedTargets.set(channelId, targets);
              }
              if (removedTargets.size > 0) {
                changeGraph((graph) => ({
                  ...graph,
                  spec: {
                    ...graph.spec,
                    channels: (graph.spec.channels ?? []).flatMap((channel) => {
                      const targets = removedTargets.get(channel.id);
                      if (!targets) return [channel];
                      const explicitTargets = channel.to ?? [];
                      if (explicitTargets.length === 0 || ![...targets].every((target) => explicitTargets.includes(target))) {
                        return [];
                      }
                      const remaining = explicitTargets.filter((target) => !targets.has(target));
                      return remaining.length > 0 ? [{ ...channel, to: remaining }] : [];
                    }),
                  },
                }));
                if (selection?.kind === 'channel' && ![...(draftGraph?.spec.channels ?? [])].some(
                  (channel) => channel.id === selection.id && (channel.to ?? []).some((target) => !removedTargets.get(channel.id)?.has(target)),
                )) {
                  setSelection(null);
                }
              }
            }}
            onSelect={(s) => {
              setSelection(s);
              if (s) setPanelOpen(true);
            }}
          />
          {selection && panelOpen && (
            <aside className="side-panel" aria-label="Details">
              <button type="button" className="icon-btn panel-close" onClick={() => setPanelOpen(false)} aria-label="Close panel">
                <Icon name="x" />
              </button>
              {selection.kind === 'node' ? (
                <NodePanel
                  summary={currentSummary}
                  status={status}
                  nodeId={selection.id}
                  onSaved={(saved) => {
                    setDraftGraph(saved.graph);
                    setDraftDirty(false);
                    onSaved(saved);
                  }}
                  onUpdateNode={(nodeId, update) =>
                    changeGraph((graph) => ({
                      ...graph,
                      spec: {
                        ...graph.spec,
                        nodes: graph.spec.nodes.map((node) => (node.id === nodeId ? { ...node, ...update } : node)),
                      },
                    }))
                  }
                  onStatus={setStatus}
                />
              ) : (
                <ChannelPanel
                  summary={currentSummary}
                  status={status}
                  channelId={selection.id}
                  onUpdateChannel={(channelId, update) =>
                    changeGraph((graph) => ({
                      ...graph,
                      spec: {
                        ...graph.spec,
                        channels: (graph.spec.channels ?? []).map((channel) =>
                          channel.id === channelId ? { ...channel, ...update } : channel,
                        ),
                      },
                    }))
                  }
                  onShowCorrelation={(c) => {
                    onCorrelation(c);
                    onTab('messages');
                  }}
                />
              )}
            </aside>
          )}
        </div>
      )}
      {tab === 'events' && <EventsView graph={summary.graph} info={info} />}
      {tab === 'files' && <FilesView summary={summary} status={status} />}
      {tab === 'messages' && (
        <MessagesExplorer graph={summary.graph} info={info} correlationId={correlationId} onCorrelation={onCorrelation} />
      )}
      {tab === 'yaml' && <YamlEditor summary={summary} onSaved={onSaved} />}
    </div>
  );
}
