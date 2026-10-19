import {TaskDecisions} from './TaskDecisions';
import {TaskActivity} from './TaskActivity';
import {useEffect, useState} from 'react';
import {api, errorMessage} from '../api/client';
import type {GraphSummary, Info} from '../api/types';
import type {TaskDraft, TaskInput, TaskReview, TaskProgress} from '../api/tasks';
import {useAsync} from '../hooks/useData';
import {Btn, ErrorBox} from './common';
import {WorkspaceHandoff} from './WorkspaceHandoff';
import {SyntheticTaskExample} from './SyntheticTaskExample';
import {TaskEvidence} from './TaskEvidence';
import {workflowTaskDefaults,updateTaskInput,outcomeRequest} from './taskDefaults';

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
    <header className="page-head"><div><h1>{create?'New task':id?'Task':'Tasks'}</h1><p className="muted">Prepare a request and review the workflow before execution.</p></div>
      {!create&&!id&&info?.identity?.role!=='viewer'&&info?.identity?.role&&<a className="btn primary" href="#/tasks/new">New task</a>}
    </header>
    {id||create ? <TaskEditor key={id??'new'} draft={detail.data} graphs={graphs.filter(g=>!g.graph.metadata.id.startsWith('task-run-'))} nativeProfile={info?.nativeProfile===true} canStart={info?.identity?.role==='admin'} canEdit={info?.identity?.role==='admin'||info?.identity?.role==='operator'}/> : <>
      {info?.identity?.role==='admin'&&<SyntheticTaskExample/>}
      <ErrorBox error={list.error}/>
      <Btn onClick={()=>void list.reload()} disabled={list.loading}>Refresh tasks</Btn>
      {!list.loading&&!list.error&&list.data?.items.length===0&&<p>No task drafts yet. Create a request using one of your existing workflows.</p>}
      <ul className="task-list">{list.data?.items.map(task=><li key={task.id}><a href={`#/tasks/${encodeURIComponent(task.id)}`}>{task.title}</a><p>{task.observedState??task.status} · Workflow {task.graphId} · Version {task.graphVersion}</p></li>)}</ul>
      {after&&<Btn onClick={()=>setAfter(undefined)}>First page</Btn>}
      {list.data?.next&&<Btn onClick={()=>setAfter(list.data?.next)}>Next page</Btn>}
    </>}
  </section>;
}
function TaskEditor({draft,graphs,canEdit,canStart,nativeProfile}:{draft?:TaskDraft;graphs:GraphSummary[];canEdit:boolean;canStart:boolean;nativeProfile:boolean}) {
  const [input,setInput] = useState<TaskInput>(()=>draft?{title:draft.title,request:draft.request,outcome:draft.outcome,graphId:draft.graphId,graphVersion:draft.graphVersion,nodeId:draft.nodeId,resultNodeId:draft.resultNodeId,projectId:draft.projectId,baseCommit:draft.baseCommit,channelId:draft.channelId,messageType:draft.messageType,payloadJson:draft.payloadJson}:empty);
  const projects=useAsync(()=>api.projects(),[]);
  const [saved,setSaved] = useState(draft);
  const [review,setReview] = useState<TaskReview>();
  const [error,setError] = useState<string>();
  const [busy,setBusy] = useState(false);
  const [confirmed,setConfirmed] = useState(false);
  const [dirty,setDirty] = useState(false);
  const selected=graphs.find(g=>g.graph.metadata.id===input.graphId)?.graph;
  useEffect(()=>{
    if(!dirty)return;
    const warn=(e:BeforeUnloadEvent)=>{e.preventDefault();e.returnValue='';};
    window.addEventListener('beforeunload',warn);return()=>window.removeEventListener('beforeunload',warn);
  },[dirty]);
  const change=(next:Partial<TaskInput>)=>{setInput(old=>updateTaskInput(old,next));setDirty(true);setReview(undefined);setConfirmed(false);};
  const save=async()=>{
    setBusy(true);setError(undefined);
    try {const result=await api.saveTask(input,saved?.id,saved?.revision);setSaved(result);setDirty(false);setReview(undefined);if(!saved)location.hash=`#/tasks/${encodeURIComponent(result.id)}`;}
    catch(e){setError(errorMessage(e));}finally{setBusy(false);}
  };
  const inspect=async()=>{
    if(!saved)return;setBusy(true);setError(undefined);
    try{setReview(await api.reviewTask(saved.id));}catch(e){setError(errorMessage(e));}finally{setBusy(false);}
  };
  const start=async()=>{
    if(!saved||!review||!confirmed)return;
    setBusy(true);setError(undefined);
    try{setSaved(await api.startTask(saved.id,review.task.revision));setReview(undefined);}
    catch(e){setError(errorMessage(e));}finally{setBusy(false);}
  };
  if(saved&&saved.status!=='draft')return <TaskTracking task={saved} canControl={canStart} canDecide={canEdit} nativeProfile={nativeProfile}/>;
  return <>
    <a href="#/tasks">Back to tasks</a>
    <form className="card form" aria-label="Task request" onSubmit={e=>{e.preventDefault();void save();}}>
      <fieldset disabled={!canEdit||busy}>
        <label>Goal<select value={input.outcome} onChange={e=>change({outcome:e.target.value,request:outcomeRequest(input,e.target.value)})}>{taskOutcomes.map(o=><option key={o.id} value={o.id}>{o.label}</option>)}</select></label>
        <p>{taskOutcomes.find(o=>o.id===input.outcome)?.description}</p>
        <label>Title<input maxLength={200} required value={input.title} onChange={e=>change({title:e.target.value})}/></label>
        <label>Request<textarea required rows={6} maxLength={16384} value={input.request} onChange={e=>change({request:e.target.value})}/></label>
        <ErrorBox error={projects.error}/>
        <label>Project<select value={input.projectId??''} onChange={e=>{const p=projects.data?.find(p=>p.id===e.target.value);change({projectId:p?.id,baseCommit:p?.head});}}>
          <option value="">No repository (managed workspace)</option>{projects.data?.map(p=><option key={p.id} value={p.id}>{p.name}</option>)}
        </select></label>
        {input.projectId&&<p>Base commit: <code>{input.baseCommit}</code>. A separate worktree will be created on the Forge server. Uncommitted source files are not copied.</p>}
        <label>Workflow<select required value={input.graphId} onChange={e=>{const g=graphs.find(g=>g.graph.metadata.id===e.target.value)?.graph;change({graphId:e.target.value,graphVersion:g?.metadata.version??0,...workflowTaskDefaults(g,input.title,input.request)});}}>
          <option value="">Select workflow</option>{!selected&&input.graphId&&<option value={input.graphId}>{input.graphId} (unavailable)</option>}
          {graphs.map(({graph:g})=><option key={g.metadata.id} value={g.metadata.id}>{g.metadata.name||g.metadata.id}</option>)}
        </select></label>
        <p className="muted">Unambiguous workflow inputs and result agents are suggested below. Review them before saving. Models and permissions are unchanged.</p>
        <label>Input delivery<select value={input.channelId??''} onChange={e=>{const c=selected?.spec.channels?.find(c=>c.id===e.target.value);change({channelId:c?.id,messageType:c?(c.types?.length===1&&!/[?*]/.test(c.types[0])?c.types[0]:''):undefined,payloadJson:c?JSON.stringify({title:input.title,description:input.request},null,2):undefined,...(c?{nodeId:c.to?.[0]??''}:{})});}}>
          <option value="">Direct agent instruction</option>{selected?.spec.channels?.filter(c=>!c.from&&c.to?.length===1&&(!c.type||c.type==='queue')&&!c.when).map(c=><option key={c.id} value={c.id}>{c.id} → {c.to?.[0]}</option>)}
        </select></label>
        {input.channelId&&<><label>Message type<input required maxLength={128} value={input.messageType??''} onChange={e=>change({messageType:e.target.value})}/></label><label>Input payload (JSON)<textarea required rows={6} maxLength={16384} value={input.payloadJson??''} onChange={e=>change({payloadJson:e.target.value})}/></label><p>The selected channel receives this JSON. Request text is retained for context; it is not sent separately. Check the workflow's expected message type and fields.</p></>}
        <label>Entry agent<select required value={input.nodeId} onChange={e=>change({nodeId:e.target.value})}>
          <option value="">Select entry agent</option>{input.nodeId&&!selected?.spec.nodes.some(n=>n.id===input.nodeId)&&<option value={input.nodeId}>{input.nodeId} (unavailable)</option>}
          {selected?.spec.nodes.filter(n=>!n.disabled).map(n=><option key={n.id} value={n.id}>{n.name||n.id}</option>)}
        </select></label>
        <label>Result agent<select value={input.resultNodeId??''} onChange={e=>change({resultNodeId:e.target.value})}>
          <option value="">Select result agent before starting</option>
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
      {review.project&&<p>Project: {review.project.name} · Source: {review.project.path} · Base: {input.baseCommit} · Task worktree: {review.workspacePath}. Commands use the workflow's configured isolation and permissions.</p>}
      <p>Execution copy: {review.executionGraphId}. Starting runs the whole workflow with a fresh managed workspace. Input goes through the selected channel or directly to the entry agent; the selected result agent supplies the result.</p>
      <p>Entry agent model: {review.model?.provider||'runtime default'} / {review.model?.name||'runtime default'}. Isolation: {review.isolation||'process'}. Process execution uses the server machine and is not a sandbox.</p>
      <p>Tool permissions remain unchanged. Every call still passes gateway policies and approvals.</p>
      <details><summary>All workflow agents and permissions</summary>{review.agents?.map(agent=><section key={agent.id}><h3>{agent.id}</h3><p>{agent.model?.provider} / {agent.model?.name} · {agent.isolation}</p><pre>{JSON.stringify(agent.grants??[],null,2)}</pre></section>)}</details>
      <label><input type="checkbox" checked={confirmed} onChange={e=>setConfirmed(e.target.checked)}/>I reviewed the workflow and authorize a separate execution copy.</label>
      {!canStart&&<p>An administrator must start this task, because it creates an execution copy.</p>}
      <Btn tone="primary" disabled={!canStart||!review.canStart||!confirmed||busy} onClick={()=>void start()}>Start task</Btn>
      <a href={`#/g/${encodeURIComponent(input.graphId)}/agent?agent=${encodeURIComponent(input.nodeId)}&section=general`}>Review agent configuration</a>
    </article>}
  </>;
}

function TaskTracking({task,canControl,canDecide,nativeProfile}:{task:TaskDraft;canControl:boolean;canDecide:boolean;nativeProfile:boolean}) {
  const [progress,setProgress]=useState<TaskProgress>();
  const [showDiagnostics,setShowDiagnostics]=useState(false);
  const [error,setError]=useState<string>();
  const [busy,setBusy]=useState(false);
  const [confirmed,setConfirmed]=useState(false);
  const [refresh,setRefresh]=useState(0);
  useEffect(()=>{
    let disposed=false;let timer:ReturnType<typeof setTimeout>;
    const load=async()=>{
      try{const result=await api.taskProgress(task.id);if(!disposed){setProgress(result);setError(undefined);}}
      catch(e){if(!disposed)setError(errorMessage(e));}
      if(!disposed)timer=setTimeout(()=>void load(),3000);
    };
    void load();return()=>{disposed=true;clearTimeout(timer);};
  },[task.id,refresh]);
  const cancel=async()=>{setBusy(true);try{await api.cancelTask(task.id);setRefresh(x=>x+1);setConfirmed(false);}catch(e){setError(errorMessage(e));}finally{setBusy(false);}};
  const current=progress?.task??task;
  const graph=current.executionGraphId;
  return <>
    <a href="#/tasks">Back to tasks</a>
    <article className="card" aria-label="Task progress">
      <h2>{task.title}</h2><p>{task.request}</p>
      <p role="status">{progress?`Task status: ${progress.state}`:'Loading progress…'}</p>
      <ErrorBox error={error}/><Btn onClick={()=>setRefresh(x=>x+1)}>Refresh task</Btn>
      {current.launchError&&<p>{current.launchError}</p>}
      {progress?.blockers.map(b=><p key={b}>{b}</p>)}
      {graph&&<TaskDecisions graphId={graph} approvals={error?undefined:progress?.pendingApprovals} canDecide={canDecide} canAdmin={canControl} onChanged={()=>setRefresh(x=>x+1)}/>}
      {progress?.workspacePath&&<WorkspaceHandoff path={progress.workspacePath} nativeProfile={nativeProfile}/>}
      <h3>Result</h3><p>{progress?.result?.summary||'No result recorded yet.'}</p>
      <p>Agent output is generated content. Inspect tool outcomes and test evidence before accepting changes.</p>
      {graph&&<TaskEvidence key={graph} graphId={graph} executions={progress?.executions??[]} role={canControl?'admin':canDecide?'operator':'viewer'}/>}
      {graph&&<nav aria-label="Task evidence"><a href={`#/g/${encodeURIComponent(graph)}/runs`}>Execution details</a> · <a href={`#/g/${encodeURIComponent(graph)}/files`}>Workspace files</a> · <a href="#/approvals">Approvals</a></nav>}
      <details><summary>Workflow and executions</summary><p>Source: {task.graphId} · Version {task.graphVersion} · Execution copy: {graph||'not prepared'}</p>
        <ul>{progress?.executions.map(x=><li key={x.id}><a href={`#/g/${encodeURIComponent(x.graphId)}/runs?execution=${encodeURIComponent(x.id)}&section=result`}>{x.nodeId}: {x.status}</a></li>)}</ul>
      </details>
      {graph&&<details onToggle={e=>setShowDiagnostics(e.currentTarget.open)}><summary>Activity and diagnostics</summary>{showDiagnostics&&<TaskActivity graphId={graph} role={canControl?'admin':canDecide?'operator':'viewer'}/>}</details>}
      {canControl&&progress?.state!=='cancelled'&&<><label><input type="checkbox" checked={confirmed} onChange={e=>setConfirmed(e.target.checked)}/>Stop this task's execution copy and retain its data.</label><Btn disabled={busy||!confirmed} onClick={()=>void cancel()}>Stop task</Btn></>}
      <p>Task completion does not itself commit, push or delete the workspace. Agent actions still depend on the workflow grants and approvals.</p>
    </article>
  </>;
}
