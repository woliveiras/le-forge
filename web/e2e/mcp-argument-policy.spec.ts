import {test,expect} from '@playwright/test';
const headers={Authorization:'Bearer e2e-admin-token'};
test('reviews exact argument restrictions and enforces them during agent execution',async({page})=>{
 await page.addInitScript(()=>localStorage.setItem('leforgeToken','e2e-admin-token'));
 await page.goto('/#/integrations');
 const form=page.getByRole('region',{name:'MCP connections',exact:true});
 await form.getByLabel('Connection ID',{exact:true}).fill('argument-policy');
 await form.getByLabel('Connection name',{exact:true}).fill('Argument policy');
 await form.getByLabel('MCP endpoint',{exact:true}).fill('https://mcp.fixture.invalid/mcp');
 await form.getByLabel('Bearer token',{exact:true}).fill('fixture-mcp-token');
 await form.getByText('Tool argument restrictions',{exact:true}).click();
 await form.getByRole('button',{name:'Add argument rule',exact:true}).click();
 await form.getByLabel('Remote tool name',{exact:true}).fill('search');
 await form.getByLabel('Argument name',{exact:true}).fill('query');
 await form.getByRole('textbox',{name:'Allowed values (one per line)',exact:true}).fill('other');
 await form.getByRole('button',{name:'Save MCP connection',exact:true}).click();
 const card=form.getByRole('article',{name:'MCP Argument policy',exact:true});
 await expect(card).toContainText('mcp.argument-policy.search');
 await expect(card).not.toContainText('mcp.argument-policy.read_issue');
 for(const mode of ['denied','allowed']){
  const id='argument-'+mode;
  if(mode==='allowed'){
   await page.goto('/#/integrations');
   await form.getByRole('button',{name:'Edit Argument policy',exact:true}).click();
   await expect(form.getByRole('textbox',{name:'Allowed values (one per line)',exact:true})).toHaveValue('other');
   await form.getByRole('textbox',{name:'Allowed values (one per line)',exact:true}).fill('issue');
   const refreshed=page.waitForResponse(r=>r.url().endsWith('/mcp/connections/argument-policy/tools/refresh')&&r.request().method()==='POST');
   await form.getByRole('button',{name:'Save MCP connection',exact:true}).click();
   expect((await refreshed).status()).toBe(200);
   await expect(card).toContainText('mcp.argument-policy.search');
  }
  const response=await page.request.post('/api/v1/graphs',{headers,data:{metadata:{id,name:id},spec:{nodes:[{id:'worker',runtime:{kind:'native'},model:{provider:'e2e',name:'fixture'},isolation:{mode:'process'},loop:{mode:'once'},prompt:'mcp-argument-policy-fixture',tools:[{name:'mcp.argument-policy.search'}]}]}}});
  expect(response.status()).toBe(201);
  await page.goto('/#/g/'+id+'/canvas');
  await page.getByRole('button',{name:'Run all',exact:true}).click();
  await expect.poll(async()=>{
   const events=await(await page.request.get('/api/v1/graphs/'+id+'/events',{headers})).json();
   return events.some((e:{type:string;data:{tool?:string;outcome?:string}})=>e.type===(mode==='allowed'?'tool.completed':'tool.denied')&&e.data.tool==='mcp.argument-policy.search'&&e.data.outcome===(mode==='allowed'?'ok':'denied'));
  }).toBe(true);
 }
 const invalid={id:'invalid-rule',name:'Invalid',service:'generic',endpoint:'https://example.com/mcp',enabled:false,argumentPolicy:[{tool:'*',argument:'repo',values:['one']}]};
 expect((await page.request.post('/api/v1/mcp/connections',{headers,data:invalid})).status()).toBe(422);
 expect((await page.request.post('/api/v1/mcp/connections',{headers:{Authorization:'Bearer e2e-viewer-token'},data:invalid})).status()).toBe(403);
});
