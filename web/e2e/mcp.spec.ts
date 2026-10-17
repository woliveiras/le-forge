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

test('executes MCP through the agent gateway and denies ungranted and disabled tools',async({page})=>{
 await page.addInitScript(()=>localStorage.setItem('leforgeToken','e2e-admin-token'));
 const create=await page.request.post('/api/v1/mcp/connections',{headers,data:{id:'runtime',name:'Runtime',service:'generic',endpoint:'https://mcp.fixture.invalid/mcp',enabled:true,credential:{value:'fixture-mcp-token'}}});expect(create.status()).toBe(201);
 const refresh=await page.request.post('/api/v1/mcp/connections/runtime/tools/refresh',{headers});expect(refresh.ok()).toBeTruthy();const connection=await refresh.json();
 for(const [id,tools] of [['mcp-allowed',[{name:'mcp.runtime.search'}]],['mcp-denied',[{name:'*'}]]] as const){
  const created=await page.request.post('/api/v1/graphs',{headers,data:{metadata:{id,name:id},spec:{nodes:[{id:'worker',runtime:{kind:'native'},model:{provider:'e2e',name:'fixture'},isolation:{mode:'process'},loop:{mode:'once'},prompt:'mcp-call-fixture',tools}]}}});expect(created.status(),await created.text()).toBe(201);
  await page.goto(`/#/g/${id}/canvas`);await page.getByRole('button',{name:'Run all',exact:true}).click();
  await expect.poll(async()=>{const res=await page.request.get(`/api/v1/graphs/${id}/events`,{headers});return (await res.json()).some((e:{type:string;data:{tool?:string;outcome?:string}})=>e.type===(id==='mcp-allowed'?'tool.completed':'tool.denied')&&e.data.tool==='mcp.runtime.search'&&e.data.outcome===(id==='mcp-allowed'?'ok':'denied'));}).toBe(true);
 }
 await expect.poll(async()=>{const res=await page.request.get('/api/v1/graphs/mcp-allowed/executions',{headers});return (await res.json()).some((x:{status:string})=>x.status==='succeeded');}).toBe(true);
 const disabled=await page.request.put('/api/v1/mcp/connections/runtime',{headers:{...headers,'If-Match':String(connection.revision)},data:{id:'runtime',name:'Runtime',service:'generic',endpoint:'https://mcp.fixture.invalid/mcp',enabled:false}});expect(disabled.ok()).toBeTruthy();
 await page.goto('/#/g/mcp-allowed/canvas');await page.getByRole('button',{name:'Stop all',exact:true}).click();await expect(page.getByRole('button',{name:'Run all',exact:true})).toBeEnabled();await page.getByRole('button',{name:'Run all',exact:true}).click();
 await expect.poll(async()=>{const res=await page.request.get('/api/v1/graphs/mcp-allowed/events',{headers});return (await res.json()).some((e:{type:string;data:{tool?:string}})=>e.type==='tool.denied'&&e.data.tool==='mcp.runtime.search');}).toBe(true);
});
