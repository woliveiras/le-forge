import { describe, expect, it } from 'vitest';
import { parseRoute, routeHash } from './App';

describe('installation destination routes', () => {
  it('supports independent destinations and safe administration fallback', () => {
    expect(parseRoute('#/models')).toEqual({ view: 'models' });
    expect(parseRoute('#/administration/network')).toEqual({ view: 'administration', section: 'network' });
    expect(parseRoute('#/administration/unknown')).toEqual({ view: 'administration', section: 'health' });
    expect(routeHash({ view: 'administration', section: 'retention' })).toBe('#/administration/retention');
    expect(routeHash({ view: 'connections' })).toBe('#/connections');
  });
  it('preserves old and new template and OAuth query payloads without interpreting them', () => {
    for (const prefix of ['integrations', 'connections']) {
      expect(parseRoute(`#/${prefix}/connect?template=github&version=1`)).toEqual({ view: 'connections', connectQuery: 'template=github&version=1', oauthQuery: undefined });
      expect(parseRoute(`#/${prefix}/oauth?state=example`)).toEqual({ view: 'connections', connectQuery: undefined, oauthQuery: 'state=example' });
      expect(parseRoute(`#/${prefix}/connect`)).toMatchObject({ connectQuery: '' });
    }
  });
  it('preserves graph correlation links', () => {
    const route = parseRoute('#/g/example/messages?c=sample');
    expect(routeHash(route)).toBe('#/g/example/messages?c=sample');
  });
});
it('round-trips connection destinations and falls back from unknown sections', () => {
 for (const hash of ['#/connections/new', '#/connections/history', '#/connections/c/team/tools', '#/connections/c/team/settings']) expect(routeHash(parseRoute(hash))).toBe(hash);
 expect(parseRoute('#/connections/c/team/unknown')).toMatchObject({connectionID:'team',connectionTab:'overview'});
});
