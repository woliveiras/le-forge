export interface Project {id:string;revision:number;name:string;path:string;head:string;createdAt:string}
import type {Approval, ValidationResult, Execution, ToolGrant, ModelRef } from './types';
export interface TaskInput {
  channelId?: string;
  messageType?: string;
  payloadJson?: string;
  projectId?: string;
  baseCommit?: string;
  title: string;
  request: string;
  outcome: string;
  graphId: string;
  graphVersion: number;
  nodeId: string;
  resultNodeId?: string;
}
export interface TaskDraft extends TaskInput {
  id: string;
  revision: number;
  status: string;
  observedState?: string;
  executionGraphId?: string;
  launchError?: string;
  createdBy: string;
  updatedBy: string;
  createdAt: string;
  updatedAt: string;
}
export interface TaskPage {items: TaskDraft[]; next?: string}
export interface TaskReview {project?:Project;workspacePath?:string;task: TaskDraft; currentGraphVersion: number; validation: ValidationResult; blockers: string[]; canStart: boolean; agents?: {id:string;model:ModelRef;grants:ToolGrant[];isolation:string}[]; executionGraphId: string; grants: ToolGrant[]; model: ModelRef; isolation: string}

export interface TaskProgress {pendingApprovals?:Approval[]|null;workspacePath?:string; task: TaskDraft; state: string; executions: Execution[]; result?: Execution; blockers: string[] }
