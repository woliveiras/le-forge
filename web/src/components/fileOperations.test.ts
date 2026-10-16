import { expect, it } from 'vitest';
import { fileOperations } from './fileOperations';
const event = { seq: 1, id: 'ev', time: '', type: 'tool.completed', executionId: 'exec', data: { tool: 'filesystem.write', args: { path: 'src/file.txt' }, outcome: 'ok' } };
it('links recorded operations to execution and matching paths without guessing shell effects', () => {
 expect(fileOperations([event], 'src')).toHaveLength(1);
 expect(fileOperations([event], 'src/file.txt')[0].executionId).toBe('exec');
 expect(fileOperations([event], 'src/file')).toHaveLength(0);
 expect(fileOperations([{...event, data:{tool:'shell.exec',args:{path:'src/file.txt'}}}], '')).toHaveLength(0);
});
it('ignores malformed or truncated audit arguments and preserves failed outcomes', () => {
 expect(fileOperations([{...event, data:null}, {...event, executionId:undefined}, {...event, data:{tool:'filesystem.write',args:'truncated…'}}], '')).toHaveLength(0);
 expect(fileOperations([{...event, data:{...event.data,outcome:'error'}}], '')[0].outcome).toBe('error');
});
it('shows observed shell changes and partial captures without inventing attribution', () => {
 const observation = {...event, type:'workspace.observed', data:{tool:'shell.exec',status:'complete',changes:[{path:'src/a',operation:'modified'},{path:'new',operation:'created'}]}};
 expect(fileOperations([observation], 'src')[0].paths).toEqual(['src/a (modified)']);
 expect(fileOperations([observation], 'missing')).toEqual([]);
 expect(fileOperations([{...observation,data:{tool:'shell.exec',status:'partial',changes:[]}}], 'missing')[0].outcome).toContain('partial');
 expect(fileOperations([{...observation,data:{tool:'shell.exec',changes:[null,{}, {path:3}]}}], '')[0].paths).toEqual([]);
});
