import { cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ProviderInfo, ProviderSettings, ProviderTestResult } from '../api/types';
import { apiError, fakeApi } from '../test/fakeApi';
import { ProvidersSection } from './ProvidersSection';

const settings = (over: Partial<ProviderSettings> = {}): ProviderSettings => ({
  revision: 4,
  default: 'mock',
  secrets: { enabled: true, keySource: 'generated' },
  types: [
    { type: 'mock', label: 'Mock', keyRequired: false },
    { type: 'openai', label: 'OpenAI', keyRequired: true, defaultBaseUrl: 'https://api.openai.com/v1' },
  ],
  ...over,
});

const mockProvider: ProviderInfo = {
  name: 'mock', type: 'mock', default: true, source: 'config', editable: false, status: 'ready',
  credential: { configured: false, source: 'none' },
};
const managed: ProviderInfo = {
  name: 'team', type: 'openai', defaultModel: 'gpt-x', source: 'managed', editable: true, status: 'ready',
  credential: { configured: true, source: 'stored', updatedAt: '2026-10-01T10:00:00Z' },
};

const okTest: ProviderTestResult = {
  provider: 'team', ok: true, checkedAt: '2026-10-13T10:00:00Z', durationMs: 300, model: 'gpt-x',
  models: { ok: true, count: 2, defaultModelListed: true },
  inference: { ok: true, reply: 'OK', durationMs: 250 },
};

beforeEach(() => {
  vi.stubGlobal('confirm', () => true);
});
afterEach(cleanup);

function card(name: string) {
  return screen.getByRole('article', { name: `Provider ${name}` });
}

describe('ProvidersSection', () => {
  it('lists providers with credential state and role-appropriate actions', async () => {
    fakeApi({
      'GET /api/v1/providers': () => [mockProvider, managed],
      'GET /api/v1/provider-settings': () => settings(),
    });
    const { rerender } = render(<ProvidersSection canEdit canTest />);
    expect(await screen.findByRole('article', { name: 'Provider team' })).toBeTruthy();
    expect(within(card('team')).getByText(/Stored encrypted in LeForge/)).toBeTruthy();
    expect(within(card('mock')).getByText('Default')).toBeTruthy();
    // Config providers are read-only; managed ones are editable.
    expect(within(card('mock')).queryByRole('button', { name: 'Edit' })).toBeNull();
    expect(within(card('team')).getByRole('button', { name: 'Edit' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Add provider' })).toBeTruthy();

    rerender(<ProvidersSection canEdit={false} canTest={false} />);
    expect(screen.queryByRole('button', { name: 'Add provider' })).toBeNull();
    expect(within(card('team')).queryByRole('button', { name: 'Test connection' })).toBeNull();
    expect(within(card('team')).queryByRole('button', { name: 'Edit' })).toBeNull();
  });

  it('adds a provider, sending the key once with the settings revision', async () => {
    const list = [mockProvider];
    const api = fakeApi({
      'GET /api/v1/providers': () => list,
      'GET /api/v1/provider-settings': () => settings(),
      'POST /api/v1/providers': (c) => {
        list.push({ ...managed, name: (c.body as { name: string }).name });
        return new Response(JSON.stringify(list[list.length - 1]), { status: 201, headers: { 'content-type': 'application/json' } });
      },
    });
    const user = userEvent.setup();
    render(<ProvidersSection canEdit canTest />);
    await user.click(await screen.findByRole('button', { name: 'Add provider' }));
    const dialog = screen.getByRole('dialog', { name: 'Add model provider' });
    await user.type(within(dialog).getByLabelText(/^Name/), 'team');
    await user.type(within(dialog).getByLabelText(/^Default model/), 'gpt-x');
    await user.type(within(dialog).getByLabelText(/^API key/), 'sk-secret-1');
    await user.click(within(dialog).getByRole('button', { name: 'Add provider' }));

    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    const [post] = api.find('POST', '/api/v1/providers');
    expect(post.headers['If-Match']).toBe('4');
    expect(post.body).toMatchObject({ name: 'team', type: 'openai', defaultModel: 'gpt-x', apiKey: { value: 'sk-secret-1' } });
    expect(await screen.findByRole('article', { name: 'Provider team' })).toBeTruthy();
    expect(document.body.textContent).not.toContain('sk-secret-1');
  });

  it('shows server validation next to the field and keeps the dialog open', async () => {
    fakeApi({
      'GET /api/v1/providers': () => [mockProvider],
      'GET /api/v1/provider-settings': () => settings(),
      'POST /api/v1/providers': () =>
        apiError(422, 'invalid provider', 'invalid', { errors: [{ path: 'apiKey', message: 'an API key is required' }] }),
    });
    const user = userEvent.setup();
    render(<ProvidersSection canEdit canTest />);
    await user.click(await screen.findByRole('button', { name: 'Add provider' }));
    const dialog = screen.getByRole('dialog');
    await user.type(within(dialog).getByLabelText(/^Name/), 'team');
    await user.click(within(dialog).getByRole('button', { name: 'Add provider' }));
    const alert = await within(dialog).findByRole('alert');
    expect(alert.textContent).toBe('an API key is required');
    expect(alert.closest('label')?.textContent).toContain('API key');
  });

  it('validates locally before sending', async () => {
    const api = fakeApi({
      'GET /api/v1/providers': () => [mockProvider],
      'GET /api/v1/provider-settings': () => settings(),
    });
    const user = userEvent.setup();
    render(<ProvidersSection canEdit canTest />);
    await user.click(await screen.findByRole('button', { name: 'Add provider' }));
    const dialog = screen.getByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Add provider' }));
    expect((await within(dialog).findByRole('alert')).textContent).toBe('Enter a name.');
    expect(api.find('POST', '/api/v1/providers')).toHaveLength(0);
  });

  it('edits without resending the stored credential, or clears it on request', async () => {
    const api = fakeApi({
      'GET /api/v1/providers': () => [mockProvider, managed],
      'GET /api/v1/provider-settings': () => settings(),
      'PUT /api/v1/providers/team': () => managed,
    });
    const user = userEvent.setup();
    render(<ProvidersSection canEdit canTest />);
    await user.click(within(await screen.findByRole('article', { name: 'Provider team' })).getByRole('button', { name: 'Edit' }));
    let dialog = screen.getByRole('dialog', { name: 'Edit team' });
    expect((within(dialog).getByLabelText(/^API key/) as HTMLInputElement).value).toBe('');
    await user.clear(within(dialog).getByLabelText(/^Default model/));
    await user.type(within(dialog).getByLabelText(/^Default model/), 'gpt-y');
    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));
    await waitFor(() => expect(api.find('PUT', '/api/v1/providers/team')).toHaveLength(1));
    expect(api.find('PUT', '/api/v1/providers/team')[0].body).not.toHaveProperty('apiKey');

    await user.click(within(card('team')).getByRole('button', { name: 'Edit' }));
    dialog = screen.getByRole('dialog', { name: 'Edit team' });
    await user.click(within(dialog).getByLabelText('Remove the stored credential'));
    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));
    await waitFor(() => expect(api.find('PUT', '/api/v1/providers/team')).toHaveLength(2));
    expect(api.find('PUT', '/api/v1/providers/team')[1].body).toMatchObject({ apiKey: { clear: true } });
  });

  it('explains a concurrent change instead of overwriting it', async () => {
    fakeApi({
      'GET /api/v1/providers': () => [mockProvider, managed],
      'GET /api/v1/provider-settings': () => settings(),
      'PUT /api/v1/providers/team': () => apiError(409, 'provider settings changed (revision 5, you edited 4); reload', 'conflict'),
    });
    const user = userEvent.setup();
    render(<ProvidersSection canEdit canTest />);
    await user.click(within(await screen.findByRole('article', { name: 'Provider team' })).getByRole('button', { name: 'Edit' }));
    const dialog = screen.getByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }));
    expect((await within(dialog).findByRole('alert')).textContent).toMatch(/changed by someone else/);
  });

  it('runs a connection test and shows inference results and failures', async () => {
    let fail = false;
    fakeApi({
      'GET /api/v1/providers': () => [mockProvider, managed],
      'GET /api/v1/provider-settings': () => settings(),
      'POST /api/v1/providers/team/test': () =>
        fail ? { ...okTest, ok: false, inference: { ok: false, error: 'llm team: HTTP 401: Incorrect API key provided: [redacted]' } } : okTest,
    });
    const user = userEvent.setup();
    render(<ProvidersSection canEdit canTest />);
    const team = await screen.findByRole('article', { name: 'Provider team' });
    await user.click(within(team).getByRole('button', { name: 'Test connection' }));
    expect((await within(team).findByRole('status')).textContent).toContain('Inference works with gpt-x');
    fail = true;
    await user.click(within(team).getByRole('button', { name: 'Test connection' }));
    await waitFor(() => expect(within(team).getByRole('status').textContent).toContain('Inference failed'));
    expect(within(team).getByRole('status').textContent).toContain('[redacted]');
    expect(within(team).getByText('Failing')).toBeTruthy();
  });

  it('tests unsaved settings from the dialog', async () => {
    const api = fakeApi({
      'GET /api/v1/providers': () => [mockProvider],
      'GET /api/v1/provider-settings': () => settings(),
      'POST /api/v1/providers/test': () => ({ ...okTest, draft: true }),
    });
    const user = userEvent.setup();
    render(<ProvidersSection canEdit canTest />);
    await user.click(await screen.findByRole('button', { name: 'Add provider' }));
    const dialog = screen.getByRole('dialog');
    await user.type(within(dialog).getByLabelText(/^Name/), 'team');
    await user.type(within(dialog).getByLabelText(/^API key/), 'sk-1');
    await user.click(within(dialog).getByRole('button', { name: 'Test settings' }));
    expect((await within(dialog).findByRole('status')).textContent).toContain('Inference works');
    expect(api.find('POST', '/api/v1/providers/test')[0].body).toMatchObject({ name: 'team', apiKey: { value: 'sk-1' } });
    expect(api.find('POST', '/api/v1/providers')).toHaveLength(0);
  });

  it('reports a provider that cannot be removed', async () => {
    fakeApi({
      'GET /api/v1/providers': () => [mockProvider, managed],
      'GET /api/v1/provider-settings': () => settings(),
      'DELETE /api/v1/providers/team': () => apiError(409, 'provider "team" is selected by graph "demo" node "writer"', 'conflict'),
    });
    const user = userEvent.setup();
    render(<ProvidersSection canEdit canTest />);
    await user.click(within(await screen.findByRole('article', { name: 'Provider team' })).getByRole('button', { name: 'Remove' }));
    expect(await within(card('team')).findByText(/node "writer"/)).toBeTruthy();
  });

  it('warns when credentials cannot be stored and disables the key field', async () => {
    fakeApi({
      'GET /api/v1/providers': () => [mockProvider],
      'GET /api/v1/provider-settings': () => settings({ secrets: { enabled: false, keySource: 'none' } }),
    });
    const user = userEvent.setup();
    render(<ProvidersSection canEdit canTest />);
    expect(await screen.findByText(/no encryption key/)).toBeTruthy();
    await user.click(screen.getByRole('button', { name: 'Add provider' }));
    expect((within(screen.getByRole('dialog')).getByLabelText(/^API key/) as HTMLInputElement).disabled).toBe(true);
  });

  it('shows unavailable providers with the server reason', async () => {
    fakeApi({
      'GET /api/v1/providers': () => [
        mockProvider,
        { ...managed, status: 'unavailable', error: 'llm: provider unavailable: team: re-enter the credential' },
      ],
      'GET /api/v1/provider-settings': () => settings(),
    });
    render(<ProvidersSection canEdit canTest />);
    const team = await screen.findByRole('article', { name: 'Provider team' });
    expect(within(team).getByText('Unavailable')).toBeTruthy();
    expect(within(team).getByText(/re-enter the credential/)).toBeTruthy();
  });

  it('surfaces load failures', async () => {
    fakeApi({
      'GET /api/v1/providers': () => apiError(500, 'internal error', 'internal'),
      'GET /api/v1/provider-settings': () => settings(),
    });
    render(<ProvidersSection canEdit canTest />);
    expect((await screen.findByRole('alert')).textContent).toContain('Could not load providers: internal error');
  });
});
