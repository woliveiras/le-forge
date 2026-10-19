import {useEffect, useState} from 'react';
import {api, errorMessage} from '../api/client';
import type {GraphSummary, Info} from '../api/types';
import type {TaskDraft, TaskInput, TaskReview} from '../api/tasks';
import {useAsync} from '../hooks/useData';
import {Btn, ErrorBox} from './common';

export const taskOutcomes = [
  {id:'understand',label:'Understand a repository',description:'Explain architecture, dependencies and areas that need attention.'},
  {id:'implement',label:'Implement a change',description:'Prepare code changes and test evidence for your review.'},
  {id:'investigate',label:'Investigate an issue',description:'Examine the supplied problem and report findings and possible fixes.'},
  {id:'review',label:'Review changes',description:'Inspect the supplied changes and explain risks and recommendations.'},
  {id:'freeform',label:'Something else',description:'Describe the work and the result you expect.'},
];
const empty: TaskInput = {title:'',request:'',outcome:'freeform',graphId:'',graphVersion:0,nodeId:''};

export function TasksPage({id,create,info,graphs}: {id?:string;create?:boolean;info?:Info;graphs:GraphSummary[]}) {
  const [after,setAfter] = useState<string>();
  const list = useAsync(()=>api.tasks(after),[after]);
  const detail = useAsync(()=>id?api.task(id):Promise.resolve(undefined),[id]);
  if (id && detail.loading) return <p className="page">Loading task…</p>;
  if (id && detail.error) return <section className="page"><a href="#/tasks">Back to tasks</a><ErrorBox error={detail.error}/><Btn onClick={()=>void detail.reload()}>Retry</Btn></section>;
  return <section className="page installation-page" aria-label="Tasks">
    <header className="page-head"><div><h1>{create?'New task':id?'Task draft':'Tasks'}</h1><p className="muted">Prepare a request and review the workflow before execution.</p></div>
      {!create&&!id&&info?.identity?.role!=='viewer'&&info?.identity?.role&&<a className="btn primary" href="#/tasks/new">New task</a>}
    </header>
    {id||create ? <TaskEditor key={id??'new'} draft={detail.data} graphs={graphs} canEdit={info?.identity?.role==='admin'||info?.identity?.role==='operator'}/> : <>
      <ErrorBox error={list.error}/>
      <Btn onClick={()=>void list.reload()} disabled={list.loading}>Refresh tasks</Btn>
      {!list.loading&&!list.error&&list.data?.items.length===0&&<p>No task drafts yet. Create a request using one of your existing workflows.</p>}
      <ul className="task-list">{list.data?.items.map(task=><li key={task.id}><a href={`#/tasks/${encodeURIComponent(task.id)}`}>{task.title}</a><p>{task.status} · Workflow {task.graphId} · Version {task.graphVersion}</p></li>)}</ul>
      {after&&<Btn onClick={()=>setAfter(undefined)}>First page</Btn>}
      {list.data?.next&&<Btn onClick={()=>setAfter(list.data?.next)}>Next page</Btn>}
    </>}
  </section>;
}
function TaskEditor({draft,graphs,canEdit}:{draft?:TaskDraft;graphs:GraphSummary[];canEdit:boolean}) {
  const [input,setInput] = useState<TaskInput>(()=>draft?{title:draft.title,request:draft.request,outcome:draft.outcome,graphId:draft.graphId,graphVersion:draft.graphVersion,nodeId:draft.nodeId}:empty);
  const [saved,setSaved] = useState(draft);
  const [review,setReview] = useState<TaskReview>();
  const [error,setError] = useState<string>();
  const [busy,setBusy] = useState(false);
  const [dirty,setDirty] = useState(false);
  const selected=graphs.find(g=>g.graph.metadata.id===input.graphId)?.graph;
  useEffect(()=>{
    if(!dirty)return;
    const warn=(e:BeforeUnloadEvent)=>{e.preventDefault();e.returnValue='';};
    window.addEventListener('beforeunload',warn);return()=>window.removeEventListener('beforeunload',warn);
  },[dirty]);
  const change=(next:Partial<TaskInput>)=>{setInput(old=>({...old,...next}));setDirty(true);setReview(undefined);};
  const save=async()=>{
    setBusy(true);setError(undefined);
    try {const result=await api.saveTask(input,saved?.id,saved?.revision);setSaved(result);setDirty(false);setReview(undefined);if(!saved)location.hash=`#/tasks/${encodeURIComponent(result.id)}`;}
    catch(e){setError(errorMessage(e));}finally{setBusy(false);}
  };
  const inspect=async()=>{
    if(!saved)return;setBusy(true);setError(undefined);
    try{setReview(await api.reviewTask(saved.id));}catch(e){setError(errorMessage(e));}finally{setBusy(false);}
  };
  return <>
    <a href="#/tasks">Back to tasks</a>
    <form className="card form" aria-label="Task request" onSubmit={e=>{e.preventDefault();void save();}}>
      <fieldset disabled={!canEdit||busy}>
        <label>Goal<select value={input.outcome} onChange={e=>change({outcome:e.target.value})}>{taskOutcomes.map(o=><option key={o.id} value={o.id}>{o.label}</option>)}</select></label>
        <p>{taskOutcomes.find(o=>o.id===input.outcome)?.description}</p>
        <label>Title<input maxLength={200} required value={input.title} onChange={e=>change({title:e.target.value})}/></label>
        <label>Request<textarea required rows={6} maxLength={16384} value={input.request} onChange={e=>change({request:e.target.value})}/></label>
        <label>Workflow<select required value={input.graphId} onChange={e=>{const g=graphs.find(g=>g.graph.metadata.id===e.target.value)?.graph;change({graphId:e.target.value,graphVersion:g?.metadata.version??0,nodeId:''});}}>
          <option value="">Select workflow</option>{!selected&&input.graphId&&<option value={input.graphId}>{input.graphId} (unavailable)</option>}
          {graphs.map(({graph:g})=><option key={g.metadata.id} value={g.metadata.id}>{g.metadata.name||g.metadata.id}</option>)}
        </select></label>
        <label>Entry agent<select required value={input.nodeId} onChange={e=>change({nodeId:e.target.value})}>
          <option value="">Select entry agent</option>{input.nodeId&&!selected?.spec.nodes.some(n=>n.id===input.nodeId)&&<option value={input.nodeId}>{input.nodeId} (unavailable)</option>}
          {selected?.spec.nodes.filter(n=>!n.disabled).map(n=><option key={n.id} value={n.id}>{n.name||n.id}</option>)}
        </select></label>
        <p className="muted">Version {input.graphVersion||'not selected'}. Saving a request does not change the workflow or grant tools.</p>
        {selected&&selected.metadata.version!==input.graphVersion&&<Btn onClick={()=>change({graphVersion:selected.metadata.version??0})}>Use current workflow version</Btn>}
      </fieldset>
      <ErrorBox error={error}/>
      <div className="row"><Btn type="submit" tone="primary" disabled={!canEdit||busy}>Save draft</Btn><Btn disabled={!saved||dirty||busy} onClick={()=>void inspect()}>Review requirements</Btn></div>
      <p role="status">{dirty?'Unsaved changes':saved?'Draft saved. No agents have been started.':'Save this request to resume it later.'}</p>
    </form>
    {review&&<article className="card" aria-label="Task requirements"><h2>Requirements</h2>
      <p>Current workflow version: {review.currentGraphVersion}</p>
      {review.blockers.map(b=><p key={b}>{b}</p>)}
      {[...(review.validation.errors??[]),...(review.validation.warnings??[])].map((v,i)=><p key={i}>{v.path}: {v.message}</p>)}
      <a href={`#/g/${encodeURIComponent(input.graphId)}/agent?agent=${encodeURIComponent(input.nodeId)}&section=general`}>Review agent configuration</a>
    </article>}
  </>;
}
