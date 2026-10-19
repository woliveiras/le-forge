import type { ValidationResult } from './types';
export interface TaskInput {
  title: string;
  request: string;
  outcome: string;
  graphId: string;
  graphVersion: number;
  nodeId: string;
}
export interface TaskDraft extends TaskInput {
  id: string;
  revision: number;
  status: 'draft';
  createdBy: string;
  updatedBy: string;
  createdAt: string;
  updatedAt: string;
}
export interface TaskPage {items: TaskDraft[]; next?: string}
export interface TaskReview {task: TaskDraft; currentGraphVersion: number; validation: ValidationResult; blockers: string[]; canStart: boolean}
