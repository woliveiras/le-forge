import {render,screen,fireEvent,waitFor,cleanup} from '@testing-library/react';
import {afterEach,beforeEach,describe,it,expect,vi} from 'vitest';
import {TaskAuthorization} from './TaskAuthorization';
import {api} from '../api/client';
vi.mock('../api/client',()=>({api:{taskAuthorization:vi.fn(),saveTaskAuthorization:vi.fn()},errorMessage:(e:Error)=>e.message}));
afterEach(cleanup);
beforeEach(()=>{vi.clearAllMocks();vi.mocked(api.taskAuthorization).mockResolvedValue({scope:'reviewed',capabilities:[{id:'worker',isolation:'process',grants:[{name:'shell.exec'}]}]});vi.mocked(api.saveTaskAuthorization).mockResolvedValue({});});
describe('Task authorization',()=>{
 it('requires an explicit confirmation and preserves scope and revision',async()=>{
 render(<TaskAuthorization taskId="tsk_1" canEdit onChanged={()=>{}}/>);
 await screen.findByText('Isolation: process');
 fireEvent.change(screen.getByLabelText('Authorization mode'),{target:{value:'full'}});
 expect(screen.getByText(/Native commands can access/)).toBeTruthy();
 expect((screen.getByRole('button',{name:'Confirm authorization'}) as HTMLButtonElement).disabled).toBe(true);
 fireEvent.click(screen.getByRole('checkbox'));fireEvent.click(screen.getByRole('button',{name:'Confirm authorization'}));
 await waitFor(()=>expect(api.saveTaskAuthorization).toHaveBeenCalledWith('tsk_1','full','reviewed',0));
 });
 it('shows failure and does not grant controls to a reader',async()=>{
 vi.mocked(api.taskAuthorization).mockRejectedValue(new Error('Offline'));
 render(<TaskAuthorization taskId="tsk_1" canEdit={false} onChanged={()=>{}}/>);
 await screen.findByText('Offline');expect(screen.queryByRole('checkbox')).toBeNull();expect(api.saveTaskAuthorization).not.toHaveBeenCalled();
 });
 it('keeps a failed save visible and never reports a change',async()=>{
 const changed=vi.fn();vi.mocked(api.saveTaskAuthorization).mockRejectedValue(new Error('Stale scope'));
 render(<TaskAuthorization taskId="tsk_1" canEdit onChanged={changed}/>);await screen.findByText('Isolation: process');
 fireEvent.click(screen.getByRole('checkbox'));fireEvent.click(screen.getByRole('button',{name:'Confirm authorization'}));await screen.findByText('Stale scope');expect(changed).not.toHaveBeenCalled();
 });
});
