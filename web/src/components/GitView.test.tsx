import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it } from 'vitest';
import { fakeApi, apiError } from '../test/fakeApi';
import { GitView } from './GitView';
afterEach(cleanup);
const result = { repository: true, directory: '/fixture/project', branch: 'main', content: ' M result.txt', attribution: 'inconclusive' };
it('inspects status, diff and history read-only with execution context', async () => {
 const backend = fakeApi({
  'GET /api/v1/graphs/g/events': () => [],
 'GET /api/v1/graphs/g/git/status': () => result,
  'GET /api/v1/graphs/g/git/diff': () => ({ ...result, content: '+updated result' }),
  'GET /api/v1/graphs/g/git/history': () => ({ ...result, content: 'abc123 baseline' }),
  'GET /api/v1/graphs/g/executions': () => [{ id: 'exec-fixture', nodeId: 'worker', replica: 0, status: 'succeeded', summary: 'Task done' }],
 });
 render(<GitView graphId="g" node="worker" replica={0} path="result.txt" />);
 expect(await screen.findByText('M result.txt')).toBeTruthy();
 expect(screen.getByText(/Change attribution is inconclusive/)).toBeTruthy();
 expect(screen.getByText(/Task done/)).toBeTruthy();
 await userEvent.selectOptions(screen.getByLabelText('Git view'), 'diff');
 expect(await screen.findByText('+updated result')).toBeTruthy();
 await userEvent.selectOptions(screen.getByLabelText('Git view'), 'history');
 expect(await screen.findByText('abc123 baseline')).toBeTruthy();
 expect(backend.calls.every(call => call.method === 'GET')).toBe(true);
 expect(String(backend.fetch.mock.calls[0][0])).toContain('replica=0');
});
it('shows Git failures and allows refresh; distinguishes non-repositories', async () => {
 let failure = true;
 fakeApi({ 'GET /api/v1/graphs/g/events': () => [],
 'GET /api/v1/graphs/g/git/status': () => failure ? apiError(500,'Git unavailable','internal') : { ...result, repository: false }, 'GET /api/v1/graphs/g/executions': () => [] });
 render(<GitView graphId="g" node="" replica={0} path="" />);
 expect(await screen.findByText('Git unavailable')).toBeTruthy();
 failure = false;
 await userEvent.click(screen.getByRole('button', {name:'Refresh Git'}));
 expect(await screen.findByText(/Workspace has no Git repository/)).toBeTruthy();
});
