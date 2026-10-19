import {test,expect} from '@playwright/test';
import {readFileSync,mkdirSync,writeFileSync} from 'node:fs';
import {join,dirname} from 'node:path';
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
 const graph={metadata:{id:'native-synthetic',name:'Synthetic native test'},spec:{nodes:[{id:'worker',runtime:{kind:'echo'},loop:{mode:'once'},workspace:{mode:'worktree'},tools:[]}]}};
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
 await page.getByRole('combobox',{name:'Workspace to review'}).selectOption(JSON.stringify(['worker',0]));
 const agentPath=await page.getByRole('region',{name:'Review task evidence'}).getByRole('region',{name:'Review workspace',exact:true}).locator('code').textContent();
 expect(agentPath).not.toBe(taskPath);writeFileSync(join(agentPath!,'tracked.txt'),'isolated agent change\n');
 await page.getByRole('combobox',{name:'Git view',exact:true}).selectOption('diff');
 await expect(page.getByRole('region',{name:'Git inspection',exact:true})).toContainText('+isolated agent change');
 expect(readFileSync(join(taskPath!,'tracked.txt'),'utf8')).toBe('changed in disposable worktree\n');
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

test('native MCP process is reviewed, discovered and repaired without granting agent tools',async({page})=>{
 const root=process.env.LEFORGE_NATIVE_E2E_DATA!;
 const token=readFileSync(join(root,'state','admin.token'),'utf8').trim();
 const headers={Authorization:`Bearer ${token}`};
 const script=join(root,'mcp-fixture.cjs');
 writeFileSync(script,`const readline=require('node:readline');
readline.createInterface({input:process.stdin}).on('line',line=>{const r=JSON.parse(line);if(!r.id)return;let result;
if(r.method==='initialize')result={protocolVersion:'2025-06-18',capabilities:{tools:{}}};
if(r.method==='tools/list')result={tools:[{name:'inspect',description:'Synthetic inspection fixture',inputSchema:{type:'object'}}]};
if(r.method==='tools/call')result={content:[{type:'text',text:'fixture'}]};
process.stdout.write(JSON.stringify({jsonrpc:'2.0',id:r.id,result})+'\\n');});`);
 const graphsBefore=await(await page.request.get('/api/v1/graphs',{headers})).json();
 await page.goto('/#/connections/new');await page.getByLabel('API token').fill(token);await page.getByRole('button',{name:'Continue',exact:true}).click();
 await page.getByLabel('Connection transport').selectOption('stdio');
 await page.getByLabel('Connection ID',{exact:true}).fill('local-fixture');await page.getByLabel('Connection name',{exact:true}).fill('Local fixture');
 await page.getByLabel('Executable path',{exact:true}).fill(join(root,'missing-executable'));await page.getByLabel('Working directory',{exact:true}).fill(root);await page.getByLabel('Process arguments',{exact:true}).fill(script);
 await page.getByLabel('Environment values (JSON)').fill('{"FIXTURE_KEY":"synthetic-native-mcp-secret"}');
 await expect(page.getByRole('button',{name:'Save MCP connection',exact:true})).toBeDisabled();
 await page.getByRole('checkbox',{name:'I reviewed this process and allow it to run on the Forge host.'}).check();await page.getByRole('button',{name:'Save MCP connection',exact:true}).click();
 await expect(page).toHaveURL(/connections\/c\/local-fixture/);await expect(page.getByText('Discovery failed: MCP process could not start')).toBeVisible();
 await page.getByRole('link',{name:'Settings',exact:true}).click();await page.getByRole('button',{name:'Edit Local fixture',exact:true}).click();
 await expect(page.getByLabel('Environment values (JSON)')).toHaveValue('');
 await page.getByLabel('Executable path',{exact:true}).fill(process.execPath);await page.getByLabel('Environment values (JSON)').fill('{}');
 await page.getByRole('checkbox',{name:'I reviewed this process and allow it to run on the Forge host.'}).check();await page.getByRole('button',{name:'Save MCP connection',exact:true}).click();
 await expect(page.getByText('1 tools discovered. Grant access to agents before they can use them.')).toBeVisible();
 await page.getByRole('link',{name:'View discovered tools',exact:true}).click();await expect(page.getByText('mcp.local-fixture.inspect',{exact:true})).toBeVisible();
 await page.reload();await expect(page.getByText('mcp.local-fixture.inspect',{exact:true})).toBeVisible();
 const publicRecord=await(await page.request.get('/api/v1/mcp/connections/local-fixture',{headers})).text();expect(publicRecord).not.toContain('synthetic-native-mcp-secret');expect(publicRecord).not.toContain('processEnvironmentSealed');
 expect(await(await page.request.get('/api/v1/graphs',{headers})).json()).toEqual(graphsBefore);
 expect(await page.evaluate(()=>JSON.stringify([localStorage,sessionStorage]))).not.toContain('synthetic-native-mcp-secret');
});

test('native task preparation failure is actionable and preserves the blocking file',async({page})=>{
 const root=process.env.LEFORGE_NATIVE_E2E_DATA!;
 const token=readFileSync(join(root,'state','admin.token'),'utf8').trim();const headers={Authorization:`Bearer ${token}`};
 const graph={metadata:{id:'native-setup-failure'},spec:{nodes:[{id:'worker',runtime:{kind:'echo'},loop:{mode:'once'},workspace:{mode:'worktree'},tools:[]}]}};
 expect((await page.request.post('/api/v1/graphs',{headers,data:graph})).status()).toBe(201);
 const response=await page.request.post('/api/v1/tasks',{headers,data:{title:'Preparation failure',request:'Synthetic setup check',outcome:'freeform',graphId:graph.metadata.id,graphVersion:1,nodeId:'worker',resultNodeId:'worker'}});expect(response.status()).toBe(201);const task=await response.json();
 const graphId=`task-run-${task.id.slice(4)}`;const obstruction=join(root,'state','workspaces','.worktrees',graphId,'worker-0');mkdirSync(dirname(obstruction),{recursive:true});writeFileSync(obstruction,'preserve');
 expect((await page.request.post(`/api/v1/tasks/${task.id}/start`,{headers:{...headers,'If-Match':String(task.revision)},data:{confirm:true}})).status()).toBe(202);
 await page.goto(`/#/tasks/${task.id}`);await page.getByLabel('API token').fill(token);await page.getByRole('button',{name:'Continue',exact:true}).click();
 await expect(page.getByRole('status')).toHaveText('Task status: blocked');await expect(page.getByText(/An agent failed during preparation/)).toBeVisible();
 await page.reload();await expect(page.getByRole('status')).toHaveText('Task status: blocked');expect(readFileSync(obstruction,'utf8')).toBe('preserve');
 expect(await(await page.request.get(`/api/v1/graphs/${graphId}/executions`,{headers})).json()).toEqual([]);
});

test('stopped native task cleanup requires review and retains its branch and source',async({page})=>{
 const root=process.env.LEFORGE_NATIVE_E2E_DATA!;
 const token=readFileSync(join(root,'state','admin.token'),'utf8').trim();const headers={Authorization:`Bearer ${token}`};
 const projects=await(await page.request.get('/api/v1/projects',{headers})).json();const project=projects.find((p:{name:string})=>p.name==='Native repository');expect(project).toBeTruthy();
 const graph={metadata:{id:'native-cleanup'},spec:{nodes:[{id:'worker',runtime:{kind:'echo'},loop:{mode:'once'},tools:[]}]}};
 expect((await page.request.post('/api/v1/graphs',{headers,data:graph})).status()).toBe(201);
 const response=await page.request.post('/api/v1/tasks',{headers,data:{title:'Reviewed cleanup',request:'Synthetic cleanup check',outcome:'freeform',graphId:graph.metadata.id,graphVersion:1,nodeId:'worker',resultNodeId:'worker',projectId:project.id,baseCommit:project.head}});expect(response.status()).toBe(201);const task=await response.json();
 expect((await page.request.post(`/api/v1/tasks/${task.id}/start`,{headers:{...headers,'If-Match':String(task.revision)},data:{confirm:true}})).status()).toBe(202);
 await page.goto(`/#/tasks/${task.id}`);await page.getByLabel('API token').fill(token);await page.getByRole('button',{name:'Continue',exact:true}).click();await expect(page.getByRole('status')).toHaveText('Task status: succeeded');
 await expect(page.getByText('Review workspace cleanup',{exact:true})).toHaveCount(0);
 await page.getByRole('checkbox',{name:"Stop this task's execution copy and retain its data."}).check();await page.getByRole('button',{name:'Stop task',exact:true}).click();await expect(page.getByRole('status')).toHaveText('Task status: cancelled');
 await page.getByText('Review workspace cleanup',{exact:true}).click();await page.getByRole('button',{name:'Preview cleanup',exact:true}).click();
 const preview=page.getByRole('region',{name:'Workspace cleanup preview'});await expect(preview).toContainText('Working tree clean.');await expect(page.getByRole('button',{name:'Remove reviewed checkout'})).toBeDisabled();
 const path=await preview.locator('code').textContent();expect(path).toContain('task-run-');
 await page.getByRole('checkbox',{name:'I stopped external writers and authorize removal of this clean checkout, keeping its branch.'}).check();await page.getByRole('button',{name:'Remove reviewed checkout'}).click();await expect(page.getByText('Checkout removed. Its branch and task history were retained.')).toBeVisible();
 expect(()=>readFileSync(join(path!,'tracked.txt'),'utf8')).toThrow();expect(readFileSync(join(project.path,'dirty.txt'),'utf8')).toBe('preserve');
 const branch=`leforge/task-run-${task.id.slice(4)}`;expect(execFileSync('git',['-C',project.path,'rev-parse','--verify',`refs/heads/${branch}`],{encoding:'utf8'}).trim()).toBe(project.head);
 await page.reload();await expect(page.getByRole('status')).toHaveText('Task status: cancelled');
});
