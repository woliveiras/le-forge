import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it } from 'vitest';
import type { Info } from '../api/types';
import { fakeApi, apiError } from '../test/fakeApi';
import { AdministrationPage, ConnectionsPage, ModelsPage } from './InstallationPages';

afterEach(cleanup);
const info = (role: string) => ({ identity: { role } } as Info);

describe('installation pages through the real API client', () => {
  it('loads only MCP resources and retains viewer read-only controls', async () => {
    const fixture = fakeApi({ 'GET /api/v1/mcp/connections': () => [] });
    render(<ConnectionsPage info={info('viewer')} />);
    expect(await screen.findByText('No MCP connections configured.')).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Save MCP connection' })).toBeNull();
    expect(screen.queryByRole('region', { name: 'Model providers' })).toBeNull();
    expect(fixture.calls.every(c => c.path.startsWith('/api/v1/mcp/'))).toBe(true);
  });
  it('shows provider errors without mounting unrelated policy forms', async () => {
    const fixture = fakeApi({
      'GET /api/v1/providers': () => apiError(503, 'Provider registry unavailable', 'unavailable'),
      'GET /api/v1/provider-settings': () => ({ revision: 1, default: '', types: [], secrets: { enabled: true } }),
    });
    render(<ModelsPage info={info('viewer')} />);
    expect(await screen.findByText(/Provider registry unavailable/)).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Add provider' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Test all providers' })).toBeNull();
    expect(fixture.calls.every(c => c.path.includes('provider'))).toBe(true);
  });
  it('isolates administration sections and permits a failed health check to recover', async () => {
    let failed = true;
    const fixture = fakeApi({
      'GET /healthz': () => ({ status: 'ok' }),
      'GET /readyz': () => failed ? apiError(502, 'Temporary outage', 'unavailable') : { storage: 'ok', queue: 'ok', sandbox: 'ok' },
      'GET /api/v1/tools': () => [{ name: 'shell.exec', capability: 'shell.exec', description: 'Run commands' }],
    });
    const view = render(<AdministrationPage info={info('viewer')} section="health" />);
    expect(await screen.findByText(/Readiness endpoint failed/)).toBeTruthy();
    failed = false;
    await userEvent.click(screen.getByRole('button', { name: 'Check services' }));
    expect(await screen.findByText(/never on the host/)).toBeTruthy();
    expect(fixture.calls.some(c => c.path.includes('network-policy'))).toBe(false);
    view.rerender(<AdministrationPage info={info('viewer')} section="tools" />);
    expect(await screen.findByText('shell.exec')).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Check services' })).toBeNull();
    expect(screen.getByRole('link', { name: 'Tool catalog' }).getAttribute('aria-current')).toBe('page');
  });
});
