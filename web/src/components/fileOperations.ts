import type { LeForgeEvent } from '../api/types';

export function fileOperations(events: LeForgeEvent[], path: string) {
  return events.flatMap(event => {
    if (event.type !== 'tool.completed' || !event.executionId || !event.data || typeof event.data !== 'object') return [];
    const data = event.data as Record<string, unknown>;
    if (!['filesystem.write', 'filesystem.edit', 'filesystem.move', 'filesystem.delete', 'filesystem.mkdir'].includes(String(data.tool))) return [];
    if (!data.args || typeof data.args !== 'object') return [];
    const args = data.args as Record<string, unknown>;
    const paths = [args.path, args.from, args.to].filter((value): value is string => typeof value === 'string');
    if (path && !paths.some(value => value === path || value.startsWith(`${path}/`))) return [];
    return [{ id: event.id, executionId: event.executionId, node: event.nodeId, replica: event.replica, tool: String(data.tool), outcome: String(data.outcome ?? 'unknown'), paths }];
  });
}
