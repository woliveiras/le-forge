import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it } from 'vitest';
import { apiError, fakeApi } from '../test/fakeApi';
import { EnvironmentProxies } from './EnvironmentProxies';
import type { ProxyView } from '../api/types';

const sample: ProxyView = { truncated: false, proxies: [{ name: 'proxy', phase: 'runtime', state: 'running', capturedAt: '2026-10-16T12:00:00Z', requests: [{ time: '2026-10-16T12:00:00Z', host: 'example.org', reason: 'host_not_allowed', outcome: 'denied', durationMs: 0 }] }] };
afterEach(cleanup);
it('loads on demand and distinguishes state from sampled blocks', async () => {
 const backend = fakeApi({ 'GET /api/v1/graphs/g/environment/proxies': () => sample });
 render(<EnvironmentProxies graphId="g" />);
 expect(backend.calls).toHaveLength(0);
 await userEvent.click(screen.getByRole('button', { name: 'Show proxy diagnostics' }));
 expect(await screen.findByText('running')).toBeTruthy();
 expect(screen.getByText('1 blocked · 0 failed · 1 sampled requests')).toBeTruthy();
 expect(screen.getByText(/example.org.*Host is not allowed/)).toBeTruthy();
 expect(screen.getByText(/not a connectivity check/)).toBeTruthy();
 await userEvent.click(screen.getByRole('button', { name: 'Hide proxy diagnostics' }));
 expect(screen.queryByText('running')).toBeNull();
});
it('shows retained preparation and unavailable logs without false zero counts', async () => {
 fakeApi({ 'GET /api/v1/graphs/g/environment/proxies': () => ({ ...sample, proxies: [{ ...sample.proxies[0], state: 'unknown', warning: 'Recent requests could not be read' }], lastPreparation: { ...sample.proxies[0], phase: 'preparation' }, truncated: true }) });
 render(<EnvironmentProxies graphId="g" />);
 await userEvent.click(screen.getByRole('button', { name: 'Show proxy diagnostics' }));
 expect(await screen.findByText('Last preparation snapshot')).toBeTruthy();
 expect(screen.getByText('Recent requests could not be read')).toBeTruthy();
 expect(screen.getByText(/first 32 proxies/)).toBeTruthy();
 expect(screen.queryByText('No blocks or failures in this sample.')).toBeNull();
});
it('hides stale status on failure and permits a refresh', async () => {
 let failed = false;
 fakeApi({ 'GET /api/v1/graphs/g/environment/proxies': () => failed ? apiError(500, 'proxy inventory unavailable', 'internal') : sample });
 render(<EnvironmentProxies graphId="g" />);
 await userEvent.click(screen.getByRole('button', { name: 'Show proxy diagnostics' }));
 await screen.findByText('running');
 failed = true;
 await userEvent.click(screen.getByRole('button', { name: 'Refresh proxy diagnostics' }));
 expect(await screen.findByText('Current proxy state could not be verified.')).toBeTruthy();
 expect(screen.queryByText('running')).toBeNull();
 failed = false;
 await userEvent.click(screen.getByRole('button', { name: 'Refresh proxy diagnostics' }));
 expect(await screen.findByText('running')).toBeTruthy();
});
it('shows absence distinctly from an offline proxy and resets scope on graph change', async () => {
 fakeApi({ 'GET /api/v1/graphs/g/environment/proxies': () => sample, 'GET /api/v1/graphs/other/environment/proxies': () => ({ proxies: [], truncated: false }) });
 const view = render(<EnvironmentProxies graphId="g" />);
 await userEvent.click(screen.getByRole('button', { name: 'Show proxy diagnostics' }));
 await screen.findByText('running');
 view.rerender(<EnvironmentProxies graphId="other" />);
 expect(await screen.findByText(/No monitored proxy containers found/)).toBeTruthy();
 expect(screen.queryByText('running')).toBeNull();
});
