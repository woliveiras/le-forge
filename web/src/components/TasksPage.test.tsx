import {cleanup,render,screen,waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {afterEach,expect,it,vi} from 'vitest';
import {TasksPage,taskOutcomes} from './TasksPage';
import {fakeApi,apiError} from '../test/fakeApi';
import type {GraphSummary,Info} from '../api/types';
const graphs=[{graph:{metadata:{id:'factory',name:'Software Factory',version:1},spec:{nodes:[{id:'worker',name:'Worker',runtime:{kind:'echo'}}]}}}] as GraphSummary[];
const info={identity:{role:'operator'}} as Info;
afterEach(()=>{cleanup();vi.restoreAllMocks();vi.unstubAllGlobals();});
it('offers distinct outcomes without an automatic target or permission choice',()=>{
 expect(new Set(taskOutcomes.map(o=>o.id)).size).toBe(5);
 fakeApi({'GET /api/v1/tasks':()=>({items:[]})});
 render(<TasksPage create info={info} graphs={graphs}/>);
 expect((screen.getByLabelText('Workflow') as HTMLSelectElement).value).toBe('');
 expect((screen.getByLabelText('Entry agent') as HTMLSelectElement).value).toBe('');
 expect(screen.queryByRole('button',{name:'Start'})).toBeNull();
});
it('preserves request on conflict and sends no tool grants',async()=>{
 const backend=fakeApi({'GET /api/v1/tasks':()=>({items:[]}), 'POST /api/v1/tasks':()=>apiError(409,'Workflow changed','conflict')});
 render(<TasksPage create info={info} graphs={graphs}/>);
 await userEvent.type(screen.getByLabelText('Title'),'Investigate a bug');
 await userEvent.type(screen.getByLabelText('Request'),'Check the parser');
 await userEvent.selectOptions(screen.getByLabelText('Workflow'),'factory');
 await userEvent.selectOptions(screen.getByLabelText('Entry agent'),'worker');
 await userEvent.click(screen.getByRole('button',{name:'Save draft'}));
 await screen.findByText('Workflow changed');
 expect((screen.getByLabelText('Request') as HTMLTextAreaElement).value).toBe('Check the parser');
 expect(backend.find('POST','/api/v1/tasks')[0].body).toEqual({title:'Investigate a bug',request:'Check the parser',outcome:'freeform',graphId:'factory',graphVersion:1,nodeId:'worker'});
 expect(backend.calls.filter(c=>c.method!=='GET')).toHaveLength(1);
});
it('renders viewer fields read-only and reports loading failures',async()=>{
 fakeApi({'GET /api/v1/tasks':()=>apiError(503,'Task storage unavailable','unavailable')});
 const {unmount}=render(<TasksPage info={{identity:{role:'viewer'}} as Info} graphs={graphs}/>);
 await screen.findByText('Task storage unavailable');
 expect(screen.queryByRole('link',{name:'New task'})).toBeNull();
 unmount();render(<TasksPage create info={{identity:{role:'viewer'}} as Info} graphs={graphs}/>);
 expect((screen.getByRole('button',{name:'Save draft'}) as HTMLButtonElement).disabled).toBe(true);
 await waitFor(()=>expect(screen.getByLabelText('Title').closest('fieldset')?.disabled).toBe(true));
});
