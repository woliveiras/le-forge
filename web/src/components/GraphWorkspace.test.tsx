import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Graph, GraphSummary, ValidationResult } from '../api/types';
import { EventStreamProvider } from '../hooks/useEventStream';
import { apiError, fakeApi, type Call } from '../test/fakeApi';
import { GraphWorkspace, type GraphTab } from './GraphWorkspace';

class FakeEventSource {
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  constructor(public url: string) {}
  addEventListener() {}
  close() {}
}

class FakeResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

const initial = (): GraphSummary => ({
  graph: {
    apiVersion: 'leforge.io/v1',
    kind: 'Graph',
    'x-editor': { zoom: 2 },
    metadata: { id: 'g', name: 'G', version: 1 },
    spec: {
      nodes: [
        { id: 'dev', name: 'Developer', prompt: 'build', runtime: { kind: 'native' }, loop: { mode: 'event' }, 'x-color': 'blue' },
        { id: 'rev', name: 'Reviewer', prompt: 'review', runtime: { kind: 'native' }, loop: { mode: 'event' } },
      ],
      channels: [
        { id: 'in', type: 'queue', to: ['dev'] },
        { id: 'code', type: 'queue', from: 'dev', to: ['rev'] },
      ],
    },
  } as Graph,
  record: { id: 'g', version: 1, state: 'stopped', desiredState: 'stopped', updatedAt: '2026-10-14T10:00:00Z' },
});

const valid: ValidationResult = { valid: true, errors: [], warnings: [] };

/** Validation that mirrors the server's prompt rule, with real field paths. */
function promptRule(graph: Graph): ValidationResult {
  const errors = graph.spec.nodes.flatMap((node, i) =>
    node.prompt?.trim() ? [] : [{ path: `spec.nodes[${i}].prompt`, message: 'native agents need a prompt' }],
  );
  return { valid: errors.length === 0, errors, warnings: [] };
}

/** A stateful fake control plane: versioned graph with If-Match checks. */
function backend(validate: (graph: Graph) => ValidationResult = promptRule) {
  const state = { current: initial() };
  const perNode = Object.fromEntries(
    ['dev', 'rev', 'agent-3'].flatMap((id) => [
      [`GET /api/v1/graphs/g/nodes/${id}/chat`, () => []],
      [`POST /api/v1/graphs/g/nodes/${id}/start`, () => status()],
    ]),
  );
  const status = () => ({ graphId: 'g', state: 'stopped', desiredState: 'stopped', nodes: [], channels: [] });
  const api = fakeApi({
    ...perNode,
    'GET /api/v1/graphs/g': () => state.current,
    'GET /api/v1/graphs/g/status': status,
    'GET /api/v1/graphs/g/events': () => [],
    'GET /api/v1/graphs/g/executions': () => [],
    'GET /api/v1/graphs/g/messages': () => [],
    'GET /api/v1/graphs/g/versions': () => [{ version: state.current.record.version, createdAt: '2026-10-14T10:00:00Z' }],
    'GET /api/v1/tools': () => [
      { name: 'filesystem.read', capability: 'filesystem.read', description: 'Read files' },
      { name: 'shell.exec', capability: 'shell.exec', description: 'Run commands' },
    ],
    'GET /api/v1/providers': () => [],
    'POST /api/v1/graphs/g/start': status,
    'POST /api/v1/graphs/validate': (call: Call) => validate(call.body as Graph),
    'PUT /api/v1/graphs/g': (call: Call) => {
      const version = state.current.record.version;
      if (call.headers['If-Match'] !== String(version)) {
        return apiError(409, `conflict: graph was modified (expected version ${call.headers['If-Match']})`, 'conflict');
      }
      const graph = call.body as Graph;
      state.current = {
        graph: { ...graph, metadata: { ...graph.metadata, version: version + 1 } },
        record: { ...state.current.record, version: version + 1 },
      };
      return state.current;
    },
  });
  return { api, state, puts: () => api.find('PUT', '/api/v1/graphs/g') };
}

function Harness({ start = 'canvas' as GraphTab }) {
  const [summary, setSummary] = useState(initial);
  const [tab, setTab] = useState<GraphTab>(start);
  return (
    <EventStreamProvider token="t">
      <GraphWorkspace summary={summary} tab={tab} onTab={setTab} onSaved={setSummary} onCorrelation={() => undefined} />
    </EventStreamProvider>
  );
}

function renderWorkspace(selected?: string) {
  location.hash = selected ? `#/g/g/canvas?sel=node:${selected}` : '#/g/g/canvas';
  return render(<Harness />);
}

const agentEditor = () => screen.getByRole('region', { name: 'Agent configuration' });
const toolbarButton = (name: string) => screen.getByRole('button', { name });

beforeEach(() => {
  vi.stubGlobal('EventSource', FakeEventSource);
  vi.stubGlobal('ResizeObserver', FakeResizeObserver);
  vi.stubGlobal('confirm', vi.fn(() => true));
});
afterEach(() => {
  cleanup();
  location.hash = '';
});

describe('GraphWorkspace editor', () => {
  it('adds an agent, edits it and saves the draft with the version it was based on', async () => {
    const user = userEvent.setup();
    const { puts } = backend();
    renderWorkspace();

    await user.click(await screen.findByRole('button', { name: 'Add agent' }));
    // The new agent is selected so it can be configured right away.
    const name = within(agentEditor()).getByLabelText('Name') as HTMLInputElement;
    expect(name.value).toBe('Agent 3');
    await user.clear(name);
    await user.type(name, 'Tester');
    await user.selectOptions(within(agentEditor()).getByLabelText('Stop policy'), 'iterations');
    await user.type(within(agentEditor()).getByLabelText('Max turns'), '3');
    await user.click(within(agentEditor()).getByLabelText('Approve each turn before it starts'));
    expect(screen.getByText('unsaved graph changes')).toBeTruthy();

    await user.click(toolbarButton('Save graph'));
    await waitFor(() => expect(puts()).toHaveLength(1));
    const put = puts()[0];
    expect(put.headers['If-Match']).toBe('1');
    const body = put.body as Graph & Record<string, unknown>;
    expect(body['x-editor']).toEqual({ zoom: 2 });
    expect(body.spec.nodes[0]).toMatchObject({ id: 'dev', 'x-color': 'blue' });
    expect(body.spec.nodes[2]).toMatchObject({
      id: 'agent-3',
      name: 'Tester',
      runtime: { kind: 'native' },
      loop: { mode: 'event', stop: { policy: 'iterations', maxIterations: 3 } },
      approval: { beforeTurn: true },
    });
    await waitFor(() => expect(screen.queryByText('unsaved graph changes')).toBeNull());
    expect(screen.getByText('v2')).toBeTruthy();
  });

  it('shows validation errors next to the agent and field, and clears them once fixed', async () => {
    const user = userEvent.setup();
    const { puts } = backend();
    renderWorkspace('dev');

    const prompt = await screen.findByRole('heading', { name: 'Prompt' });
    const section = prompt.closest('section')!;
    await user.click(within(section).getByRole('button', { name: 'Edit' }));
    await user.clear(within(section).getByRole('textbox'));
    await user.click(within(section).getByRole('button', { name: 'Apply to draft' }));
    await user.click(toolbarButton('Save graph'));

    const summary = await screen.findByRole('list', { name: 'Graph validation' });
    expect(within(summary).getByText(/1 error must be fixed/)).toBeTruthy();
    expect(within(summary).getByText('prompt: native agents need a prompt')).toBeTruthy();
    expect(within(section).getByText('native agents need a prompt')).toBeTruthy();
    expect(screen.getByLabelText('1 validation errors')).toBeTruthy(); // marker on the canvas agent
    expect(puts()).toHaveLength(0);

    // Fixing the field revalidates without another save attempt.
    await user.click(within(section).getByRole('button', { name: 'Edit' }));
    await user.type(within(section).getByRole('textbox'), 'build it');
    await user.click(within(section).getByRole('button', { name: 'Apply to draft' }));
    await waitFor(() => expect(screen.queryByRole('list', { name: 'Graph validation' })).toBeNull(), { timeout: 2000 });

    await user.click(toolbarButton('Save graph'));
    await waitFor(() => expect(puts()).toHaveLength(1));
    expect((puts()[0].body as Graph).spec.nodes[0].prompt).toBe('build it');
  });

  it('shows errors the server reports on save against their element', async () => {
    const user = userEvent.setup();
    const b = backend(() => valid);
    renderWorkspace('rev');
    await user.type(within(await screen.findByRole('region', { name: 'Agent configuration' })).getByLabelText('Name'), '!');
    const route = b.api.fetch.getMockImplementation()!;
    b.api.fetch.mockImplementation(async (input, init) =>
      init?.method === 'PUT'
        ? apiError(422, 'invalid graph', 'invalid', {
            valid: false,
            errors: [{ path: 'spec.nodes[1].colour', message: 'unknown field "colour"' }],
            warnings: [],
          })
        : route(input, init),
    );
    await user.click(toolbarButton('Save graph'));
    expect(await screen.findByText('colour: unknown field "colour"')).toBeTruthy();
    expect(within(screen.getByRole('list', { name: 'Agent validation' })).getByText('unknown field "colour"')).toBeTruthy();
    expect(screen.getByText('unsaved graph changes')).toBeTruthy();
  });

  it('keeps local edits on a version conflict and lets the user overwrite or reload', async () => {
    const user = userEvent.setup();
    const { state, puts } = backend(() => valid);
    renderWorkspace('dev');
    const name = within(await screen.findByRole('region', { name: 'Agent configuration' })).getByLabelText('Name');
    await user.clear(name);
    await user.type(name, 'Mine');

    // Someone else saves version 5 meanwhile.
    state.current = { ...state.current, record: { ...state.current.record, version: 5 } };
    state.current.graph = { ...state.current.graph, spec: { ...state.current.graph.spec, nodes: [{ ...state.current.graph.spec.nodes[0], name: 'Theirs' }, state.current.graph.spec.nodes[1]] } };

    await user.click(toolbarButton('Save graph'));
    expect(await screen.findByText(/Someone saved a newer version/)).toBeTruthy();
    expect(screen.getByText('unsaved graph changes')).toBeTruthy();
    expect((within(agentEditor()).getByLabelText('Name') as HTMLInputElement).value).toBe('Mine');

    await user.click(toolbarButton('Keep mine and overwrite'));
    await waitFor(() => expect(puts()).toHaveLength(2));
    expect(puts()[1].headers['If-Match']).toBe('5');
    expect((puts()[1].body as Graph).spec.nodes[0].name).toBe('Mine');
    await waitFor(() => expect(screen.queryByText(/Someone saved a newer version/)).toBeNull());

    // Reload path: discard local edits in favour of the latest version.
    await user.clear(within(agentEditor()).getByLabelText('Name'));
    await user.type(within(agentEditor()).getByLabelText('Name'), 'Again');
    state.current = { ...state.current, record: { ...state.current.record, version: 9 } };
    await user.click(toolbarButton('Save graph'));
    await user.click(await screen.findByRole('button', { name: 'Reload latest' }));
    await waitFor(() => expect((within(agentEditor()).getByLabelText('Name') as HTMLInputElement).value).toBe('Mine'));
    expect(screen.queryByText('unsaved graph changes')).toBeNull();
  });

  it('confirms what removing an agent does before changing the draft', async () => {
    const user = userEvent.setup();
    const confirmSpy = vi.fn(() => false);
    vi.stubGlobal('confirm', confirmSpy);
    const { puts } = backend(() => valid);
    renderWorkspace('dev');
    await screen.findByRole('region', { name: 'Agent configuration' });

    await user.click(toolbarButton('Remove agent'));
    expect(confirmSpy).toHaveBeenCalledWith('Remove agent "Developer" and channels "in", "code"?');
    expect(screen.queryByText('unsaved graph changes')).toBeNull();

    confirmSpy.mockReturnValue(true);
    await user.click(toolbarButton('Remove agent'));
    expect(screen.getByText('unsaved graph changes')).toBeTruthy();
    await user.click(toolbarButton('Save graph'));
    await waitFor(() => expect(puts()).toHaveLength(1));
    const body = puts()[0].body as Graph;
    expect(body.spec.nodes.map((n) => n.id)).toEqual(['rev']);
    expect(body.spec.channels).toEqual([]);
  });

  it('discards the draft back to the saved version', async () => {
    const user = userEvent.setup();
    backend(() => valid);
    renderWorkspace('dev');
    const name = within(await screen.findByRole('region', { name: 'Agent configuration' })).getByLabelText('Name');
    await user.type(name, ' changed');
    await user.click(toolbarButton('Discard'));
    expect((within(agentEditor()).getByLabelText('Name') as HTMLInputElement).value).toBe('Developer');
    expect(screen.queryByText('unsaved graph changes')).toBeNull();
  });

  it('configures the command sandbox and warns about shell access under process isolation', async () => {
    const user = userEvent.setup();
    const { puts } = backend(() => valid);
    renderWorkspace('dev');
    const editor = await screen.findByRole('region', { name: 'Agent configuration' });

    // Granting shell under process isolation explains the risk.
    expect(within(editor).queryByText(/can run shell commands/)).toBeNull();
    await user.click(await screen.findByRole('checkbox', { name: /shell\.exec/ }));
    expect(within(editor).getByText(/can run shell commands/)).toBeTruthy();

    await user.selectOptions(within(editor).getByLabelText('Isolation'), 'docker');
    expect(within(editor).queryByText(/can run shell commands/)).toBeNull();
    await user.selectOptions(within(editor).getByLabelText('Network'), 'bridge');
    await user.type(within(editor).getByLabelText('Memory limit'), '2g');

    await user.click(toolbarButton('Save graph'));
    await waitFor(() => expect(puts()).toHaveLength(1));
    const dev = (puts()[0].body as Graph).spec.nodes[0];
    expect(dev.isolation).toEqual({ mode: 'docker', network: 'bridge', memory: '2g' });
    expect(dev.tools).toEqual([{ name: 'shell.exec' }]);
  });

  it('asks before starting while graph edits are unsaved', async () => {
    const user = userEvent.setup();
    const confirmSpy = vi.fn(() => false);
    vi.stubGlobal('confirm', confirmSpy);
    const { api } = backend(() => valid);
    renderWorkspace('dev');
    await user.type(within(await screen.findByRole('region', { name: 'Agent configuration' })).getByLabelText('Name'), 'x');
    await user.click(screen.getByRole('button', { name: /Run all/ }));
    expect(confirmSpy).toHaveBeenCalledWith(expect.stringContaining('unsaved graph changes are not included'));
    expect(api.find('POST', '/api/v1/graphs/g/start')).toHaveLength(0);
  });
});

describe('GraphWorkspace YAML view', () => {
  it('edits the same draft as the canvas, in both directions', async () => {
    const user = userEvent.setup();
    const { puts } = backend(() => valid);
    renderWorkspace('dev');
    const name = within(await screen.findByRole('region', { name: 'Agent configuration' })).getByLabelText('Name');
    await user.clear(name);
    await user.type(name, 'From canvas');

    await user.click(screen.getByRole('tab', { name: 'Definition' }));
    const yaml = (await screen.findByRole('textbox', { name: 'Graph YAML' })) as HTMLTextAreaElement;
    expect(yaml.value).toContain('name: From canvas');
    expect(yaml.value).toContain('x-color: blue');

    fireEvent.change(yaml, { target: { value: yaml.value.replace('name: From canvas', 'name: From YAML') } });
    // Unparseable text keeps the last valid draft.
    fireEvent.change(yaml, { target: { value: `${yaml.value}\n  bad: [` } });
    expect(screen.getByText(/The graph view and Save use the last valid YAML/)).toBeTruthy();

    await user.click(screen.getByRole('tab', { name: 'Graph' }));
    expect((within(agentEditor()).getByLabelText('Name') as HTMLInputElement).value).toBe('From YAML');

    await user.click(toolbarButton('Save graph'));
    await waitFor(() => expect(puts()).toHaveLength(1));
    const body = puts()[0].body as Graph & Record<string, unknown>;
    expect(body.spec.nodes[0].name).toBe('From YAML');
    expect(body['x-editor']).toEqual({ zoom: 2 });
  });
});
