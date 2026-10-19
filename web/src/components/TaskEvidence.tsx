import {useState} from 'react';
import type {Execution} from '../api/types';
import {Btn} from './common';
import {GitView} from './GitView';

export function TaskEvidence({graphId,executions}:{graphId:string;executions:Execution[]}) {
  const [inspect,setInspect]=useState(false);
  return <section aria-label="Review task evidence">
    <h3>Changes and checks</h3>
    <p>Inspect the current working tree, diff and history. A completed agent run is not proof that tests passed or that changes were committed or pushed.</p>
    <Btn onClick={()=>setInspect(value=>!value)}>{inspect?'Hide changes':'Inspect changes'}</Btn>
    {inspect&&<GitView key={graphId} graphId={graphId} node="" replica={0} path=""/>}
    <h4>Check execution evidence</h4>
    <p>Open recorded tool calls to review commands and their outcomes. No test result is inferred from the agent summary.</p>
    {executions.length===0?<p>No execution evidence recorded yet.</p>:<ul>{executions.map(run=><li key={run.id}><a href={`#/g/${encodeURIComponent(graphId)}/runs?execution=${encodeURIComponent(run.id)}&section=tools`}>{run.nodeId} · {run.status} · Tool calls</a></li>)}</ul>}
  </section>;
}
