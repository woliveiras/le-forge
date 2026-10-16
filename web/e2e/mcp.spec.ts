import { expect,test } from '@playwright/test';
const headers={Authorization:'Bearer e2e-admin-token'};
test('registers a remote MCP, discovers tools and persists exact agent permissions',async({page})=>{
 await page.addInitScript(()=>localStorage.setItem('leforgeToken','e2e-admin-token'));
 await page.goto('/#/integrations');const section=page.getByRole('region',{name:'MCP connections',exact:true});
 await section.getByLabel('Connection ID',{exact:true}).fill('team');await section.getByLabel('Connection name',{exact:true}).fill('Team MCP');await section.getByLabel('MCP endpoint',{exact:true}).fill('https://mcp.fixture.invalid/mcp');await section.getByLabel('Bearer token',{exact:true}).fill('fixture-mcp-token');
 await section.getByRole('button',{name:'Save MCP connection',exact:true}).click();
 await section.getByRole('button',{name:'Refresh tools for Team MCP',exact:true}).click();await expect(section).toContainText('mcp.team.search');await expect(section).toContainText('mcp.team.read_issue');
 const response=await page.request.get('/api/v1/mcp/connections',{headers});expect(response.ok()).toBeTruthy();expect(await response.text()).not.toContain('fixture-mcp-token');
 await page.goto('/#/g/journal-capture/canvas?sel=node:worker');
 const grants=page.getByRole('region',{name:'MCP permissions',exact:true});
 await grants.getByRole('checkbox',{name:'mcp.team.search',exact:true}).check();await Promise.all([page.waitForResponse(r=>r.url().endsWith('/api/v1/graphs/journal-capture')&&r.request().method()==='PUT'&&r.status()===200),page.getByRole('button',{name:'Save graph',exact:true}).click()]);
 await expect(page.getByRole('button',{name:'Save graph',exact:true})).toBeDisabled();await page.reload();await expect(grants.getByRole('checkbox',{name:'mcp.team.search',exact:true})).toBeChecked();
 const graph=await page.request.get('/api/v1/graphs/journal-capture',{headers});const saved=await graph.json();expect(saved.graph.spec.nodes[0].tools).toContainEqual({name:'mcp.team.search'});
 await page.goto('/#/integrations');await section.getByRole('button',{name:'Edit Team MCP',exact:true}).click();await expect(section.getByLabel('Bearer token',{exact:true})).toHaveValue('');await section.getByLabel('Connection enabled',{exact:true}).uncheck();await section.getByRole('button',{name:'Save MCP connection',exact:true}).click();await expect(section.getByRole('button',{name:'Refresh tools for Team MCP',exact:true})).toBeDisabled();
 await page.goto('/#/g/journal-capture/canvas?sel=node:worker');await expect(grants).toContainText('Unavailable in the current catalogue');await grants.getByRole('checkbox',{name:/mcp.team.search/}).click();await expect(grants.getByRole('checkbox',{name:/mcp.team.search/})).toHaveCount(0);await Promise.all([page.waitForResponse(r=>r.url().endsWith('/api/v1/graphs/journal-capture')&&r.request().method()==='PUT'&&r.status()===200),page.getByRole('button',{name:'Save graph',exact:true}).click()]);await expect(page.getByRole('button',{name:'Save graph',exact:true})).toBeDisabled();
});
test('reports MCP authentication failure without a discovered tool catalogue',async({page})=>{
 await page.addInitScript(()=>localStorage.setItem('leforgeToken','e2e-admin-token'));await page.goto('/#/integrations');const section=page.getByRole('region',{name:'MCP connections',exact:true});
 await section.getByLabel('Connection ID',{exact:true}).fill('noauth');await section.getByLabel('Connection name',{exact:true}).fill('No auth');await section.getByLabel('MCP endpoint',{exact:true}).fill('https://mcp.fixture.invalid/mcp');await section.getByRole('button',{name:'Save MCP connection',exact:true}).click();await section.getByRole('button',{name:'Test No auth',exact:true}).click();
 await expect(section.getByRole('article',{name:'MCP No auth',exact:true})).toContainText('Discovery failed');
});
