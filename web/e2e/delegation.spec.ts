import {test,expect} from '@playwright/test';
const admin={Authorization:'Bearer e2e-admin-token'};
const viewer={Authorization:'Bearer e2e-viewer-token'};
for(const decision of ['approve','reject'] as const) {
 test('delegation '+decision+' preserves grants and returns the outcome to the parent',async({page})=>{
  const id='delegation-'+decision;
  const graph={metadata:{id,name:id},spec:{nodes:[{id:'parent',runtime:{kind:'native'},prompt:'delegation-fixture',model:{provider:'e2e',name:'fixture'},loop:{mode:'once'},tools:[{name:'delegation.request'},{name:'filesystem.read',paths:['docs/**'] }]}]}};
  expect((await page.request.post('/api/v1/graphs',{headers:admin,data:graph})).status()).toBe(201);
  await page.addInitScript(()=>localStorage.setItem('leforgeToken','e2e-admin-token'));
  await page.goto('/#/g/'+id+'/canvas');
  await page.getByRole('button',{name:'Run all',exact:true}).click();
  let approval:{id:string;executionId:string;detail:{parentExecutionId:string}} | undefined;
  await expect.poll(async()=>{
   const approvals=await (await page.request.get('/api/v1/approvals?graph='+id,{headers:admin})).json();
   approval=approvals.find((a:{kind:string;status:string})=>a.kind==='delegation'&&a.status==='pending');
   return !!approval;
  }).toBe(true);
  const childId=approval!.executionId,parentId=approval!.detail.parentExecutionId;
  const pending=await (await page.request.get('/api/v1/executions/'+childId,{headers:admin})).json();
  expect(pending.status).toBe('awaiting_approval');expect(pending.summary).toBeUndefined();
  expect((await page.request.post('/api/v1/approvals/'+approval!.id+'/approve',{headers:viewer,data:{}})).status()).toBe(403);
  await page.getByRole('button',{name:/^Approvals/}).click();
  const card=page.locator('.approval').filter({hasText:id});
  await expect(card).toContainText('tool-free analysis');
  await expect(card).toContainText('"effectiveTools": []');
  await card.getByRole('button',{name:decision==='approve'?'Approve':'Reject',exact:true}).click();
  await expect.poll(async()=>(await (await page.request.get('/api/v1/executions/'+parentId,{headers:admin})).json()).status).toBe('succeeded');
  await page.goto('/#/g/'+id+'/runs?execution='+childId+'&section=result');
  await expect(page.getByRole('complementary',{name:'Delegation context'})).toContainText('No tools or workspace access');
  await page.getByText('Delegation audit',{exact:true}).click();
  await expect(page.getByRole('complementary',{name:'Delegation context'})).toContainText('Decision: '+(decision==='approve'?'approved':'rejected'));
  await expect(page.getByRole('complementary',{name:'Delegation context'})).toContainText('Effective grants: None');
  if(decision==='approve') await expect(page.getByRole('region',{name:'Run detail'})).toContainText('Delegated fixture result');
  else await expect(page.getByRole('region',{name:'Run detail'})).toContainText('delegation rejected or expired');
  await page.getByRole('link',{name:'View parent execution'}).click();
  await expect(page.getByRole('region',{name:'Run detail'})).toContainText(decision==='approve'?'Delegated fixture result':'delegation rejected or expired');
  const saved=await (await page.request.get('/api/v1/graphs/'+id,{headers:admin})).json();
  expect(saved.record.version).toBe(1);expect(saved.graph.spec.nodes).toHaveLength(1);expect(saved.graph.spec.nodes[0].tools).toEqual(graph.spec.nodes[0].tools);
 });
}
