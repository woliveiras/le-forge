import { useRef, useState } from 'react';
import { Btn } from './common';
import type { Info } from '../api/types';
import { MCPSection } from './MCPSection';
import { ProvidersSection, type ProvidersHandle } from './ProvidersSection';
import { IntegrationsView } from './IntegrationsView';
import { NetworkPolicySection } from './NetworkPolicySection';
import { HistoryRetentionSection } from './HistoryRetentionSection';
import { SharedEnvironmentsSection } from './SharedEnvironmentsSection';
import { ToolCatalog } from './ToolCatalog';

export const administrationSections = ['health', 'network', 'resources', 'retention', 'tools'] as const;
export type AdministrationSection = typeof administrationSections[number];
const titles: Record<AdministrationSection, string> = {
  health: 'System health', network: 'Network policies', resources: 'Shared images & caches',
  retention: 'History retention', tools: 'Tool catalog',
};

export function ConnectionsPage({ info, connectQuery, oauthQuery }: { info?: Info; connectQuery?: string; oauthQuery?: string }) {
  return <div className="page installation-page"><header className="page-head"><div><h1>Connections</h1>
    <p className="muted">Manage installation MCP connections. Agent tool permissions are granted separately.</p></div></header>
    <MCPSection key={connectQuery ?? 'manual'} role={info?.identity?.role} connectQuery={connectQuery} oauthQuery={oauthQuery} />
  </div>;
}

export function ModelsPage({ info }: { info?: Info }) {
  const providers = useRef<ProvidersHandle>(null);
  const [testing, setTesting] = useState(false);
  const testAll = async () => {
    setTesting(true);
    try { await providers.current?.testAll(); } finally { setTesting(false); }
  };
  const role = info?.identity?.role;
  return <div className="page installation-page"><header className="page-head"><div><h1>Models</h1>
    <p className="muted">Configure model providers for this installation, then choose a model in each agent.</p></div>
    {(role === 'admin' || role === 'operator') && <Btn onClick={() => void testAll()} disabled={testing}>{testing ? 'Testing providers…' : 'Test all providers'}</Btn>}</header>
    <ProvidersSection ref={providers} canEdit={role === 'admin'} canTest={role === 'admin' || role === 'operator'} />
  </div>;
}

export function AdministrationPage({ info, infoError, section }: { info?: Info; infoError?: string; section: AdministrationSection }) {
  const canManage = info?.identity?.role === 'admin';
  return <div className="page administration-page"><header className="page-head"><div><h1>Administration</h1>
    <p className="muted">Installation-wide services and policies. Changes may affect multiple graphs.</p></div></header>
    <nav className="section-navigation" aria-label="Administration sections">{administrationSections.map(item =>
      <a key={item} href={`#/administration/${item}`} aria-current={section === item ? 'page' : undefined}>{titles[item]}</a>)}</nav>
    <div key={section} className="administration-content">
      {section === 'health' && <IntegrationsView info={info} infoError={infoError} />}
      {section === 'network' && <NetworkPolicySection canManage={canManage} />}
      {section === 'resources' && <SharedEnvironmentsSection canManage={canManage} />}
      {section === 'retention' && <HistoryRetentionSection canManage={canManage} />}
      {section === 'tools' && <ToolCatalog />}
    </div>
  </div>;
}
