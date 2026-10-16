import type { LeForgeEvent } from '../api/types';

export function fileOperations(events: LeForgeEvent[], path: string) {
  return events.flatMap(event => {
    if (!['tool.completed', 'workspace.observed'].includes(event.type) || !event.executionId || !event.data || typeof event.data !== 'object') return [];
    const data = event.data as Record<string, unknown>;
    if (event.type === 'workspace.observed') {
      const changes = Array.isArray(data.changes) ? data.changes.filter((value): value is { path: string; operation: string } =>
        !!value && typeof value === 'object' && typeof value.path === 'string' && ['created', 'modified', 'deleted'].includes(value.operation)) : [];
      const matching = changes.filter(change => !path || change.path === path || change.path.startsWith(`${path}/`));
      if (path && matching.length === 0 && data.status !== 'partial') return [];
      return [{ id: event.id, executionId: event.executionId, node: event.nodeId, replica: event.replica, tool: String(data.tool), outcome: `observed · ${data.status === 'complete' ? 'complete' : 'partial'}`, paths: matching.map(change => `${change.path} (${change.operation})`) }];
    }
    if (!['filesystem.write', 'filesystem.edit', 'filesystem.move', 'filesystem.delete', 'filesystem.mkdir'].includes(String(data.tool))) return [];
    if (!data.args || typeof data.args !== 'object') return [];
    const args = data.args as Record<string, unknown>;
    const paths = [args.path, args.from, args.to].filter((value): value is string => typeof value === 'string');
    if (path && !paths.some(value => value === path || value.startsWith(`${path}/`))) return [];
    return [{ id: event.id, executionId: event.executionId, node: event.nodeId, replica: event.replica, tool: String(data.tool), outcome: String(data.outcome ?? 'unknown'), paths }];
  });
}
