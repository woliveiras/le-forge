import {test,expect} from '@playwright/test';
import {readFileSync,mkdirSync,writeFileSync} from 'node:fs';
import {join} from 'node:path';
import {execFileSync} from 'node:child_process';
test('native profile authenticates, registers a local repo and executes a reviewed task in a worktree',async({page})=>{
 const root=process.env.LEFORGE_NATIVE_E2E_DATA!;
 // Credential belongs only to the disposable native installation created by the wrapper.
 const token=readFileSync(join(root,'state','admin.token'),'utf8').trim();
 const headers={Authorization:`Bearer ${token}`};
 const repo=join(root,'repo');mkdirSync(repo);execFileSync('git',['init','-q',repo]);writeFileSync(join(repo,'tracked.txt'),'original\n');execFileSync('git',['-C',repo,'add','tracked.txt']);execFileSync('git',['-C',repo,'-c','user.name=Fixture','-c','user.email=fixture@example.test','commit','--allow-empty','-qm','initial']);writeFileSync(join(repo,'dirty.txt'),'preserve');
 const initial=await(await page.request.get('/api/v1/graphs',{headers})).json();
 expect(initial).toHaveLength(1);expect(initial[0].graph.metadata.id).toBe('software-factory');expect(initial[0].record.state).toBe('stopped');
 expect(initial[0].graph.spec.nodes.every((node:{runtime:{kind:string}})=>node.runtime.kind==='native')).toBe(true);
 const graph={metadata:{id:'native-synthetic',name:'Synthetic native test'},spec:{nodes:[{id:'worker',runtime:{kind:'echo'},loop:{mode:'once'},tools:[]}]}};
 expect((await page.request.post('/api/v1/graphs',{headers,data:graph})).status()).toBe(201);
 await page.goto('/#/projects');
 await page.getByLabel('API token').fill(token);
 await page.getByRole('button',{name:'Continue',exact:true}).click();
 await expect(page.getByRole('heading',{name:'Projects',exact:true})).toBeVisible();
 await page.getByRole('textbox',{name:'Project name',exact:true}).fill('Native repository');await page.getByRole('textbox',{name:'Repository path',exact:true}).fill(repo);await page.getByRole('checkbox').check();await page.getByRole('button',{name:'Register project',exact:true}).click();
 await page.getByRole('link',{name:'Create task',exact:true}).click();
 await expect(page.getByRole('combobox',{name:'Workflow',exact:true})).toContainText('Software Factory');
 await page.getByRole('textbox',{name:'Title',exact:true}).fill('Synthetic native worktree task');await page.getByRole('textbox',{name:'Request',exact:true}).fill('Test plumbing only; no real model or account access.');
 await page.getByRole('combobox',{name:'Project',exact:true}).selectOption({label:'Native repository'});await page.getByRole('combobox',{name:'Workflow',exact:true}).selectOption('native-synthetic');await page.getByRole('combobox',{name:'Entry agent',exact:true}).selectOption('worker');await page.getByRole('combobox',{name:'Result agent',exact:true}).selectOption('worker');
 await page.getByRole('button',{name:'Save draft',exact:true}).click();await page.getByRole('button',{name:'Review requirements'}).click();await page.getByRole('checkbox').check();await page.getByRole('button',{name:'Start task',exact:true}).click();
 await expect(page.getByRole('status')).toHaveText('Task status: succeeded');
 await page.reload();await expect(page.getByRole('status')).toHaveText('Task status: succeeded');
 expect(readFileSync(join(repo,'dirty.txt'),'utf8')).toBe('preserve');
 await expect(page.getByRole('link',{name:'Open in VS Code',exact:true})).toHaveCount(0);
 await page.getByRole('checkbox',{name:'Forge and this browser are on the same computer, with VS Code installed.'}).check();
 await expect(page.getByRole('link',{name:'Open in VS Code',exact:true})).toHaveAttribute('href',/^vscode:\/\/file.*task-run-/);
 // Inspect the handoff target without launching an external editor during tests.
 const taskPath=await page.getByRole('region',{name:'Review workspace',exact:true}).locator('code').textContent();
 expect(taskPath?.startsWith(join(root,'state','workspaces'))).toBe(true);
 writeFileSync(join(taskPath!,'tracked.txt'),'changed in disposable worktree\n');
 await page.getByRole('button',{name:'Inspect changes',exact:true}).click();
 await expect(page.getByRole('region',{name:'Git inspection',exact:true})).toContainText('M tracked.txt');
 await page.getByRole('combobox',{name:'Git view',exact:true}).selectOption('diff');
 await expect(page.getByRole('region',{name:'Git inspection',exact:true})).toContainText('+changed in disposable worktree');
 expect(readFileSync(join(repo,'tracked.txt'),'utf8')).toBe('original\n');

 await page.getByRole('link',{name:'Workspace files',exact:true}).click();await expect(page).toHaveURL(/task-run-.*\/files/);
});

test('native first-use example needs no external account or model',async({page})=>{
 const root=process.env.LEFORGE_NATIVE_E2E_DATA!;
 const token=readFileSync(join(root,'state','admin.token'),'utf8').trim();
 await page.goto('/#/tasks');await page.getByLabel('API token').fill(token);await page.getByRole('button',{name:'Continue',exact:true}).click();
 await page.getByRole('button',{name:'Preview example'}).click();
 await page.getByRole('checkbox',{name:'Create the synthetic workflow and task draft.'}).check();await page.getByRole('button',{name:'Prepare example'}).click();
 await expect(page.getByRole('status')).toHaveText('Draft saved. No agents have been started.');
 await page.getByRole('button',{name:'Review requirements'}).click();
 await page.getByRole('checkbox',{name:'I reviewed the workflow and authorize a separate execution copy.'}).check();await page.getByRole('button',{name:'Start task',exact:true}).click();
 await expect(page.getByRole('status')).toHaveText('Task status: succeeded');
});

test('preinstalled factory supplies editable task inputs without creating another workflow',async({page})=>{
 const root=process.env.LEFORGE_NATIVE_E2E_DATA!;
 const token=readFileSync(join(root,'state','admin.token'),'utf8').trim();
 const headers={Authorization:`Bearer ${token}`};
 const before=await(await page.request.get('/api/v1/graphs/software-factory',{headers})).json();
 await page.goto('/#/tasks/new');await page.getByLabel('API token').fill(token);await page.getByRole('button',{name:'Continue',exact:true}).click();
 await page.getByRole('combobox',{name:'Workflow',exact:true}).selectOption('software-factory');
 await expect(page.getByRole('combobox',{name:'Entry agent',exact:true})).toHaveValue('planner');
 await expect(page.getByRole('combobox',{name:'Result agent',exact:true})).toHaveValue('integrator');
 await expect(page.getByRole('combobox',{name:'Input delivery',exact:true})).toHaveValue('requests');
 await expect(page.getByRole('textbox',{name:'Message type',exact:true})).toHaveValue('feature.request');
 await page.getByRole('textbox',{name:'Title',exact:true}).fill('Factory draft');await page.getByRole('textbox',{name:'Request',exact:true}).fill('Inspect a future change');
 expect(JSON.parse(await page.getByRole('textbox',{name:'Input payload (JSON)',exact:true}).inputValue())).toEqual({title:'Factory draft',description:'Inspect a future change'});
 await page.getByRole('button',{name:'Save draft',exact:true}).click();await expect(page).toHaveURL(/#\/tasks\/tsk_/);await expect(page.getByRole('status')).toHaveText('Draft saved. No agents have been started.');await page.reload();
 await expect(page.getByRole('combobox',{name:'Entry agent',exact:true})).toHaveValue('planner');
 await page.getByRole('button',{name:'Review requirements'}).click();await expect(page.getByRole('button',{name:'Start task',exact:true})).toBeDisabled();
 const after=await(await page.request.get('/api/v1/graphs/software-factory',{headers})).json();expect(after).toEqual(before);
 expect(await(await page.request.get('/api/v1/graphs/software-factory/executions',{headers})).json()).toEqual([]);
});
