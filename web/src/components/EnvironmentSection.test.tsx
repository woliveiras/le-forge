import { cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { EnvironmentView, Graph } from '../api/types';
import { EventStreamProvider } from '../hooks/useEventStream';
import { apiError, fakeApi } from '../test/fakeApi';
import { EnvironmentSection, environmentUsers } from './EnvironmentSection';

class FakeEventSource {
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  addEventListener() {}
  close() {}
}

const graph = (env?: Graph['spec']['environment']): Graph => ({
  apiVersion: 'leforge.io/v1',
  kind: 'Graph',
  metadata: { id: 'g', name: 'G', version: 2 },
  spec: {
    environment: env,
    nodes: [
      { id: 'builder', isolation: { mode: 'docker' } },
      { id: 'own-image', isolation: { mode: 'docker', image: 'alpine' } },
      { id: 'local' },
    ],
  },
});

const view = (over: Partial<EnvironmentView> = {}): EnvironmentView => ({
  graph: 'g', state: 'absent', defined: true, currentHash: 'abc123', upToDate: false, ...over,
});

function renderSection(props: Partial<Parameters<typeof EnvironmentSection>[0]> = {}) {
  const env = { image: 'golang:1.26', setup: ['go mod download'] };
  const onChange = vi.fn();
  render(
    <EventStreamProvider token="t">
      <EnvironmentSection
        graph={graph(env)}
        savedGraph={graph(env)}
        dirty={false}
        running={false}
        canBuild
        canReset
        onChange={onChange}
        {...props}
      />
    </EventStreamProvider>,
  );
  return { onChange, status: () => screen.getByRole('region', { name: 'Environment status' }) };
}

beforeEach(() => {
  vi.stubGlobal('EventSource', FakeEventSource);
  vi.stubGlobal('confirm', vi.fn(() => true));
});
afterEach(cleanup);

describe('EnvironmentSection', () => {
  it('lists the agents that use the environment', () => {
    expect(environmentUsers(graph())).toEqual(['builder']);
  });

  it('builds a saved definition and shows the result', async () => {
    const user = userEvent.setup();
    const api = fakeApi({
      'GET /api/v1/graphs/g/environment': () => view(),
      'POST /api/v1/graphs/g/environment/build': () =>
        view({ state: 'ready', upToDate: true, hash: 'abc123', image: 'leforge-env/g:abc123', finishedAt: '2026-10-14T10:00:00Z', actor: 'admin', log: '==> Environment ready' }),
    });
    const { status } = renderSection();
    expect(await within(status()).findByText('Not built')).toBeTruthy();
    expect(within(status()).getByText('Used by: builder.')).toBeTruthy();
    await user.click(within(status()).getByRole('button', { name: 'Build' }));
    expect(api.find('POST', '/api/v1/graphs/g/environment/build')[0].body).toEqual({ force: false });
    expect(await within(status()).findByText('Ready')).toBeTruthy();
    expect(within(status()).getByText('leforge-env/g:abc123')).toBeTruthy();
    // An up-to-date environment offers a forced rebuild.
    await user.click(within(status()).getByRole('button', { name: 'Rebuild' }));
    expect(api.find('POST', '/api/v1/graphs/g/environment/build')[1].body).toEqual({ force: true });
  });

  it('shows failures with the build log', async () => {
    fakeApi({
      'GET /api/v1/graphs/g/environment': () =>
        view({ state: 'failed', hash: 'abc123', error: 'setup: exit status 1: go: not found', log: '$ go mod download\nsh: go: not found' }),
    });
    const { status } = renderSection();
    expect(await within(status()).findByText('Build failed')).toBeTruthy();
    expect(within(status()).getByText('setup: exit status 1: go: not found')).toBeTruthy();
    expect(within(status()).getByLabelText('Environment build log').textContent).toContain('sh: go: not found');
    expect(within(status()).getByRole('button', { name: 'Build again' })).toBeTruthy();
  });

  it('warns when the image is outdated or the definition is unsaved', async () => {
    fakeApi({ 'GET /api/v1/graphs/g/environment': () => view({ state: 'ready', hash: 'old', upToDate: false, image: 'leforge-env/g:old' }) });
    const edited = graph({ image: 'golang:1.26', setup: ['go mod download', 'go vet ./...'] });
    const { status } = renderSection({ graph: edited, dirty: true });
    expect(await within(status()).findByText('Outdated')).toBeTruthy();
    expect(within(status()).getByText(/Unsaved environment changes/)).toBeTruthy();
    expect((within(status()).getByRole('button', { name: 'Build' }) as HTMLButtonElement).disabled).toBe(true);
  });

  it('applies roles and refuses reset while the graph runs', async () => {
    const api = fakeApi({
      'GET /api/v1/graphs/g/environment': () => view({ state: 'ready', upToDate: true, hash: 'abc123', image: 'leforge-env/g:abc123' }),
      'POST /api/v1/graphs/g/environment/reset': () => apiError(409, 'graph g is running; stop it before resetting its environment', 'conflict'),
    });
    const { status } = renderSection({ canBuild: false, canReset: true, running: true });
    await within(status()).findByText('Ready');
    expect(within(status()).queryByRole('button', { name: /Build|Rebuild/ })).toBeNull();
    const reset = within(status()).getByRole('button', { name: 'Reset' }) as HTMLButtonElement;
    expect(reset.disabled).toBe(true);
    expect(within(status()).getByText('Stop the graph to reset its environment.')).toBeTruthy();
    cleanup();

    renderSection({ canBuild: false, canReset: false });
    await waitFor(() => expect(api.find('GET', '/api/v1/graphs/g/environment').length).toBeGreaterThan(1));
    expect(screen.queryByRole('button', { name: 'Reset' })).toBeNull();
  });

  it('edits the definition in the draft', async () => {
    const user = userEvent.setup();
    fakeApi({ 'GET /api/v1/graphs/g/environment': () => view({ defined: false }) });
    const onChange = vi.fn();
    render(
      <EventStreamProvider token="t">
        <EnvironmentSection graph={graph()} savedGraph={graph()} dirty={false} running={false} canBuild canReset onChange={onChange} />
      </EventStreamProvider>,
    );
    await user.selectOptions(screen.getByLabelText('Source'), 'image');
    const apply = (n: number) => onChange.mock.calls[n][0](graph()) as Graph;
    expect(apply(0).spec.environment).toEqual({ image: '' });
  });
});
