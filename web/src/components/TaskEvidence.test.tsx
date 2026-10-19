import {cleanup,render,screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {afterEach,expect,it,vi} from 'vitest';
import {TaskEvidence} from './TaskEvidence';
import {fakeApi,apiError} from '../test/fakeApi';
import type {Execution} from '../api/types';
afterEach(()=>{cleanup();vi.restoreAllMocks();vi.unstubAllGlobals();});
it('loads Git evidence only on request and keeps every call read-only',async()=>{
 const backend=fakeApi({'GET /api/v1/graphs/task-run-fixture/git/status':()=>({repository:true,directory:'/fixture',branch:'task',content:' M file.txt'}),'GET /api/v1/graphs/task-run-fixture/events':()=>[],'GET /api/v1/graphs/task-run-fixture/executions':()=>[]});
 render(<TaskEvidence graphId="task-run-fixture" executions={[{id:'exec-fixture',nodeId:'worker',status:'succeeded'} as Execution]}/>);
 expect(backend.calls).toHaveLength(0);expect(screen.getByRole('link',{name:/worker.*Tool calls/}).getAttribute('href')).toBe('#/g/task-run-fixture/runs?execution=exec-fixture&section=tools');
 await userEvent.click(screen.getByRole('button',{name:'Inspect changes'}));await screen.findByText('M file.txt');
 expect(backend.calls.every(c=>c.method==='GET')).toBe(true);
});
it('shows unavailable evidence without inventing a passing test result',async()=>{
 fakeApi({'GET /api/v1/graphs/g/git/status':()=>apiError(503,'Git temporarily unavailable','unavailable'),'GET /api/v1/graphs/g/events':()=>[],'GET /api/v1/graphs/g/executions':()=>[]});
 render(<TaskEvidence graphId="g" executions={[]}/>);await userEvent.click(screen.getByRole('button',{name:'Inspect changes'}));
 await screen.findByText('Git temporarily unavailable');expect(screen.getByText('No execution evidence recorded yet.')).toBeTruthy();expect(screen.getByText(/not proof that tests passed/)).toBeTruthy();
});
