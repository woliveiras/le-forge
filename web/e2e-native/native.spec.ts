import {test,expect} from '@playwright/test';
import {readFileSync,mkdirSync,writeFileSync} from 'node:fs';
import {join} from 'node:path';
import {execFileSync} from 'node:child_process';
test('native profile authenticates, registers a local repo and executes a reviewed task in a worktree',async({page})=>{
 const root=process.env.LEFORGE_NATIVE_E2E_DATA!;
 // Credential belongs only to the disposable native installation created by the wrapper.
 const token=readFileSync(join(root,'state','admin.token'),'utf8').trim();
 const headers={Authorization:`Bearer ${token}`};
 const repo=join(root,'repo');mkdirSync(repo);execFileSync('git',['init','-q',repo]);execFileSync('git',['-C',repo,'-c','user.name=Fixture','-c','user.email=fixture@example.test','commit','--allow-empty','-qm','initial']);writeFileSync(join(repo,'dirty.txt'),'preserve');
 const graph={metadata:{id:'native-synthetic',name:'Synthetic native test'},spec:{nodes:[{id:'worker',runtime:{kind:'echo'},loop:{mode:'once'},tools:[]}]}};
 expect((await page.request.post('/api/v1/graphs',{headers,data:graph})).status()).toBe(201);
 await page.goto('/#/projects');
 await page.getByLabel('API token').fill(token);
 await page.getByRole('button',{name:'Continue',exact:true}).click();
 await expect(page.getByRole('heading',{name:'Projects',exact:true})).toBeVisible();
 await page.getByRole('textbox',{name:'Project name',exact:true}).fill('Native repository');await page.getByRole('textbox',{name:'Repository path',exact:true}).fill(repo);await page.getByRole('checkbox').check();await page.getByRole('button',{name:'Register project',exact:true}).click();
 await page.getByRole('link',{name:'Create task',exact:true}).click();
 await page.getByRole('textbox',{name:'Title',exact:true}).fill('Synthetic native worktree task');await page.getByRole('textbox',{name:'Request',exact:true}).fill('Test plumbing only; no real model or account access.');
 await page.getByRole('combobox',{name:'Project',exact:true}).selectOption({label:'Native repository'});await page.getByRole('combobox',{name:'Workflow',exact:true}).selectOption('native-synthetic');await page.getByRole('combobox',{name:'Entry agent',exact:true}).selectOption('worker');await page.getByRole('combobox',{name:'Result agent',exact:true}).selectOption('worker');
 await page.getByRole('button',{name:'Save draft',exact:true}).click();await page.getByRole('button',{name:'Review requirements'}).click();await page.getByRole('checkbox').check();await page.getByRole('button',{name:'Start task',exact:true}).click();
 await expect(page.getByRole('status')).toHaveText('Task status: succeeded');
 await page.reload();await expect(page.getByRole('status')).toHaveText('Task status: succeeded');
 expect(readFileSync(join(repo,'dirty.txt'),'utf8')).toBe('preserve');
 await page.getByRole('link',{name:'Workspace files',exact:true}).click();await expect(page).toHaveURL(/task-run-.*\/files/);
});
