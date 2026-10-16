import { expect, test } from '@playwright/test';

test('runs a task then inspects its files, diff, history and execution context', async ({ page }) => {
 await page.addInitScript(() => localStorage.setItem('leforgeToken', 'e2e-admin-token'));
 await page.goto('/#/g/git-inspect/canvas');
 await page.getByRole('button', { name: 'Run all', exact: true }).click();
 await expect.poll(async () => {
  const response = await page.request.get('/api/v1/graphs/git-inspect/executions', { headers: { Authorization: 'Bearer e2e-admin-token' } });
  return (await response.json()).some((run: {status:string}) => run.status === 'succeeded');
 }).toBe(true);
 await page.getByRole('tab', { name: 'Files', exact: true }).click();
 const git = page.getByRole('region', {name:'Git inspection'});
 await expect(git).toContainText('M result.txt');
 await expect(git).toContainText('Inspection task finished');
 await expect(git).toContainText('filesystem.write');
 await expect(git).toContainText('shell.exec');
 await expect(git).toContainText('shell.txt (created)');
 await expect(git).toContainText('Change attribution is inconclusive');
 await page.getByRole('button', {name:/result.txt/}).click();
 await expect(page.getByRole('region', {name:'File contents'})).toContainText('updated by fixture task');
 await git.getByLabel('Git view').selectOption('diff');
 await expect(git).toContainText('+updated by fixture task');
 await expect(git).toContainText('-baseline result');
 await git.getByLabel('Git view').selectOption('history');
 await expect(git).toContainText('inspection baseline');
 for (const endpoint of ['files','files/content','git/diff']) {
  const response = await page.request.delete(`/api/v1/graphs/git-inspect/${endpoint}`, {headers:{Authorization:'Bearer e2e-admin-token'}});
  expect(response.status()).toBe(405);
 }
});

test('read-only agent cannot write through filesystem or shell despite its grants', async ({page}) => {
 await page.addInitScript(() => localStorage.setItem('leforgeToken', 'e2e-admin-token'));
 await page.goto('/#/g/git-inspect-readonly/canvas');
 await page.getByRole('button', {name:'Run all',exact:true}).click();
 await expect.poll(async () => {
  const response = await page.request.get('/api/v1/graphs/git-inspect-readonly/events?types=tool.denied', {headers:{Authorization:'Bearer e2e-admin-token'}});
  const events = await response.json();
  return events.filter((event:{data:{error?:string}}) => event.data.error?.includes('read-only')).length;
 }).toBe(2);
 await page.getByRole('tab', {name:'Files',exact:true}).click();
 await expect(page.getByRole('region',{name:'Git inspection'})).toContainText('Working tree clean.');
 await page.getByRole('button', {name:/result.txt/}).click();
 await expect(page.getByRole('region',{name:'File contents'})).toContainText('baseline result');
});
