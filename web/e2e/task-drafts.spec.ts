import {test,expect} from '@playwright/test';
const headers={Authorization:'Bearer e2e-admin-token'};
test('saves and resumes a reviewed request without starting or changing the workflow',async({page})=>{
 const graph={metadata:{id:'task-draft-flow',name:'Task draft flow'},spec:{nodes:[{id:'worker',name:'Worker',runtime:{kind:'echo'},loop:{mode:'once'},tools:[]}]}};
 expect((await page.request.post('/api/v1/graphs',{headers,data:graph})).status()).toBe(201);
 await page.addInitScript(()=>localStorage.setItem('leforgeToken','e2e-admin-token'));
 await page.goto('/#/tasks');
 await page.getByRole('link',{name:'New task',exact:true}).click();
 await page.getByRole('combobox',{name:'Goal',exact:true}).selectOption('investigate');
 await page.getByRole('textbox',{name:'Title',exact:true}).fill('Investigate parser');
 await page.getByRole('textbox',{name:'Request',exact:true}).fill('Explain why the parser rejects this input.');
 await page.getByRole('combobox',{name:'Workflow',exact:true}).selectOption('task-draft-flow');
 await page.getByRole('combobox',{name:'Entry agent',exact:true}).selectOption('worker');
 await page.getByRole('button',{name:'Save draft',exact:true}).click();
 await expect(page).toHaveURL(/#\/tasks\/tsk_/);
 await expect(page.getByRole('status')).toHaveText('Draft saved. No agents have been started.');
 await page.reload();
 await expect(page.getByRole('textbox',{name:'Request',exact:true})).toHaveValue('Explain why the parser rejects this input.');
 await page.getByRole('button',{name:'Review requirements'}).click();
 await expect(page.getByRole('article',{name:'Task requirements'})).toContainText('does not start agents');
 expect(await (await page.request.get('/api/v1/graphs/task-draft-flow/executions',{headers})).json()).toEqual([]);
 const saved=await (await page.request.get('/api/v1/graphs/task-draft-flow',{headers})).json();
 expect(saved.record.version).toBe(1);expect(saved.graph.spec.nodes[0].tools??[]).toEqual([]);
 const url=page.url();
 await page.getByRole('textbox',{name:'Title',exact:true}).fill('Updated request');
 await page.getByRole('button',{name:'Save draft',exact:true}).click();
 await expect(page.getByRole('status')).toHaveText('Draft saved. No agents have been started.');
 await page.reload();await expect(page.getByRole('textbox',{name:'Title',exact:true})).toHaveValue('Updated request');
 await page.getByRole('link',{name:'Back to tasks',exact:true}).click();
 await page.getByRole('link',{name:'Updated request',exact:true}).click();await expect(page).toHaveURL(url);
});

test('viewer cannot save a task through the UI or API',async({page})=>{
 await page.addInitScript(()=>localStorage.setItem('leforgeToken','e2e-viewer-token'));
 await page.goto('/#/tasks/new');
 await expect(page.getByRole('button',{name:'Save draft'})).toBeDisabled();
 expect((await page.request.post('/api/v1/tasks',{headers:{Authorization:'Bearer e2e-viewer-token'},data:{}})).status()).toBe(403);
});
