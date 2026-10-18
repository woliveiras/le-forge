import type {MCPConnectionInput, MCPArgumentRule} from '../api/types';
import {Btn} from './common';

export function MCPArgumentPolicyFields({input,onChange}:{input:MCPConnectionInput;onChange:(value:MCPConnectionInput)=>void}) {
 const rules=input.argumentPolicy??[];
 const change=(next:MCPArgumentRule[])=>onChange({...input,argumentPolicy:next.length?next:undefined});
 const update=(index:number,patch:Partial<MCPArgumentRule>)=>change(rules.map((rule,i)=>i===index?{...rule,...patch}:rule));
 return <details open={rules.length>0||undefined}><summary>Tool argument restrictions</summary>
  <p>Optional exact matches for top-level string arguments. Once any rule is added, only listed tools are eligible. Every rule for a tool must match. Agent grants and approvals still apply.</p>
  <p>Use non-secret resource identifiers from the discovered schema. Values are case-sensitive, with no wildcards, URL resolution or project/space ownership checks. This does not filter returned content or limit query costs.</p>
  {rules.map((rule,index)=><fieldset key={index}><legend>Argument rule {index+1}</legend>
   <label>Remote tool name<input value={rule.tool} onChange={e=>update(index,{tool:e.target.value})} required maxLength={128}/></label>
   <label>Argument name<input value={rule.argument} onChange={e=>update(index,{argument:e.target.value})} required maxLength={128}/></label>
   <label>Allowed values (one per line)<textarea value={rule.values.join('\n')} onChange={e=>update(index,{values:e.target.value.split('\n')})} required/></label>
   <Btn onClick={()=>change(rules.filter((_,i)=>i!==index))}>Remove rule {index+1}</Btn>
  </fieldset>)}
  <Btn disabled={rules.length>=64} onClick={()=>change([...rules,{tool:'',argument:'',values:[]}])}>Add argument rule</Btn>
  {rules.length>0&&<p>Saving policy changes clears discovery. Review the refreshed catalogue; tools with missing or non-string argument schemas are excluded.</p>}
 </details>;
}
