import { useState } from 'react';
import { api, errorMessage } from '../api/client';
import type { EnvironmentDiscovery as Discovery, EnvironmentSpec } from '../api/types';
import { Btn, ErrorBox } from './common';

/** Discovery and review do not save the graph or start a build. */
export function EnvironmentDiscovery({ graphId, disabled, onAccept }: {
  graphId: string;
  disabled: boolean;
  onAccept: (environment: EnvironmentSpec) => void;
}) {
  const [result, setResult] = useState<Discovery>();
  const [selected, setSelected] = useState('');
  const [service, setService] = useState('');
  const [target, setTarget] = useState('');
  const [preview, setPreview] = useState<EnvironmentSpec>();
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);
  const candidate = result?.candidates.find((item) => item.path === selected);

  const discover = async () => {
    setBusy(true);
    setError(undefined);
    setPreview(undefined);
    setResult(undefined);
    setSelected('');
    setService(''); setTarget('');
    try {
      const next = await api.discoverEnvironment(graphId);
      setResult(next);
    } catch (cause) { setError(errorMessage(cause)); }
    finally { setBusy(false); }
  };
  const review = async () => {
    if (!candidate || !result) return;
    setBusy(true);
    setError(undefined);
    setPreview(undefined);
    try {
      setPreview(await api.resolveEnvironment(graphId, {
        source: result.source, path: candidate.path, sha256: candidate.sha256,
        ...(service ? { service } : {}),
        ...(target ? { target } : {}),
      }));
    } catch (cause) { setError(errorMessage(cause)); }
    finally { setBusy(false); }
  };

  return <section aria-label="Repository environment">
    <h4>Discover from repository</h4>
    <p className="muted small">Inspect the existing checkout, select a definition and review it before accepting. Accepting updates the draft; save the graph before building.</p>
    {disabled && <p className="muted small">Save workspace changes and remove any saved or draft environment before discovering a repository definition.</p>}
    <Btn small disabled={disabled || busy} onClick={() => void discover()}>{busy ? 'Checking…' : 'Discover environment'}</Btn>
    <ErrorBox error={error} />
    {result?.status === 'configured' && <p>An environment is already saved. Remove it and save the graph before discovering another.</p>}
    {result?.status === 'proposal_required' && <p>No environment definition found. Agent proposals are not available yet; configure an image or Dockerfile manually.</p>}
    {!!result?.candidates.length && <>
      <p className="muted small">Source: {result.source}</p>
      <label>Repository definition
        <select disabled={busy || disabled} value={selected} onChange={(event) => {
          setSelected(event.target.value); setService(''); setTarget(''); setPreview(undefined); setError(undefined);
        }}>
          <option value="">Select a definition</option>
          {result.candidates.map((item) => <option key={item.path} value={item.path}>{item.path}</option>)}
        </select>
      </label>
      <ErrorBox error={candidate?.error} />
      {!!candidate?.services?.length && <label>Development service
        <select disabled={busy || disabled} value={service} onChange={(event) => {
          setService(event.target.value); setPreview(undefined); setError(undefined);
        }}>
          <option value="">Select a service</option>
          {candidate.services.map((name) => <option key={name} value={name}>{name}</option>)}
        </select>
      </label>}
      {!!candidate?.targets?.length && <label>Build stage
        <select disabled={busy || disabled} value={target} onChange={(event) => {
          setTarget(event.target.value); setPreview(undefined); setError(undefined);
        }}>
          <option value="">Final stage (default)</option>
          {candidate.targets.map((name) => <option key={name} value={name}>{name}</option>)}
        </select>
      </label>}
      <Btn small disabled={disabled || busy || !candidate || !!candidate.error || (result.source === 'compose' && !service)} onClick={() => void review()}>Review environment</Btn>
    </>}
    {preview && <>
      <p>Review the image, setup commands and network settings below. Nothing has been saved or built.</p>
      <pre aria-label="Environment proposal">{JSON.stringify({ ...preview, origin: undefined }, null, 2)}</pre>
      <Btn small disabled={disabled || busy} onClick={() => { onAccept(preview); setPreview(undefined); setResult(undefined); }}>Accept into draft</Btn>
    </>}
  </section>;
}
