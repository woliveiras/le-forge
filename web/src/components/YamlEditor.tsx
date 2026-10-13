import { useEffect, useState } from 'react';
import { parse, stringify } from 'yaml';
import { api, errorMessage } from '../api/client';
import type { Graph, GraphSummary, ValidationResult } from '../api/types';
import { useAsync } from '../hooks/useData';
import { Btn, ErrorBox, fmtDateTime } from './common';

/** Edit the graph document as YAML: validate, save with optimistic concurrency, browse versions. */
export function YamlEditor({ summary, onSaved }: { summary: GraphSummary; onSaved: (s: GraphSummary) => void }) {
  const gid = summary.graph.metadata.id;
  const [text, setText] = useState(() => stringify(summary.graph));
  const [dirty, setDirty] = useState(false);
  const [result, setResult] = useState<ValidationResult>();
  const [error, setError] = useState<string>();
  const [saved, setSaved] = useState<string>();
  const versions = useAsync(() => api.graphVersions(gid), [gid, summary.record.version]);

  useEffect(() => {
    if (!dirty) setText(stringify(summary.graph));
  }, [summary, dirty]);

  const parsed = (): Graph | undefined => {
    try {
      return parse(text) as Graph;
    } catch (e) {
      setError(`YAML: ${(e as Error).message}`);
      return undefined;
    }
  };

  const validate = async () => {
    setSaved(undefined);
    const g = parsed();
    if (!g) return;
    try {
      setResult(await api.validateGraph(g));
      setError(undefined);
    } catch (e) {
      setError(errorMessage(e));
    }
  };

  const save = async () => {
    setSaved(undefined);
    const g = parsed();
    if (!g) return;
    try {
      const s = await api.updateGraph(gid, g, summary.record.version);
      onSaved(s);
      setDirty(false);
      setResult(undefined);
      setError(undefined);
      setSaved(`Saved version ${s.record.version}`);
    } catch (e) {
      setError(errorMessage(e));
    }
  };

  const loadVersion = async (v: number) => {
    try {
      const s = await api.getGraph(gid, v);
      setText(stringify(s.graph));
      setDirty(true);
      setSaved(`Loaded version ${v} into the editor – save to restore it as a new version.`);
    } catch (e) {
      setError(errorMessage(e));
    }
  };

  return (
    <div className="page fill split">
      <div className="split-main">
        <div className="page-head">
          <h2>Definition</h2>
          <span className="muted small">
            version {summary.record.version}
            {dirty ? ' · edited' : ''}
          </span>
          <Btn small onClick={validate}>
            Validate
          </Btn>
          <Btn small tone="primary" onClick={save} disabled={!dirty}>
            Save
          </Btn>
          <Btn
            small
            tone="ghost"
            disabled={!dirty}
            onClick={() => {
              setText(stringify(summary.graph));
              setDirty(false);
            }}
          >
            Revert
          </Btn>
          <Btn small tone="ghost" onClick={() => void download(gid).catch((e) => setError(errorMessage(e)))}>
            Export
          </Btn>
        </div>
        <ErrorBox error={error} />
        {saved && <div className="ok small">{saved}</div>}
        {result && (
          <div className={`validation ${result.valid ? 'ok' : 'bad'}`}>
            <strong>{result.valid ? 'Valid' : 'Invalid'}</strong>
            <ul>
              {(result.errors ?? []).map((e, i) => (
                <li key={`e${i}`} className="bad">
                  <code>{e.path}</code> {e.message}
                </li>
              ))}
              {(result.warnings ?? []).map((e, i) => (
                <li key={`w${i}`} className="warn">
                  <code>{e.path}</code> {e.message}
                </li>
              ))}
            </ul>
          </div>
        )}
        <textarea
          className="code-editor mono"
          spellCheck={false}
          value={text}
          onChange={(e) => {
            setText(e.target.value);
            setDirty(true);
          }}
          onKeyDown={(e) => {
            if (e.key === 'Tab') {
              e.preventDefault();
              const t = e.currentTarget;
              const s = t.selectionStart;
              const v = t.value.slice(0, s) + '  ' + t.value.slice(t.selectionEnd);
              setText(v);
              setDirty(true);
              requestAnimationFrame(() => t.setSelectionRange(s + 2, s + 2));
            }
            if (e.key === 's' && (e.metaKey || e.ctrlKey)) {
              e.preventDefault();
              void save();
            }
          }}
          aria-label="Graph YAML"
        />
      </div>
      <aside className="split-side narrow">
        <h3>Versions</h3>
        <ErrorBox error={versions.error} />
        <ul className="versions">
          {[...(versions.data ?? [])].reverse().map((v) => (
            <li key={v.version}>
              <button type="button" className="link" onClick={() => loadVersion(v.version)}>
                v{v.version}
              </button>{' '}
              <span className="muted small">
                {fmtDateTime(v.createdAt)}
                {v.author ? ` · ${v.author}` : ''}
              </span>
              {v.version === summary.record.version && <span className="small"> (current)</span>}
            </li>
          ))}
        </ul>
      </aside>
    </div>
  );
}

/** The export endpoint needs the bearer token, so download via fetch + blob rather than a plain link. */
async function download(gid: string) {
  const text = await api.exportGraph(gid, 'yaml');
  const url = URL.createObjectURL(new Blob([text], { type: 'application/yaml' }));
  const a = document.createElement('a');
  a.href = url;
  a.download = `${gid}.yaml`;
  a.click();
  URL.revokeObjectURL(url);
}
