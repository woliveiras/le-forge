export interface DelegationPolicy {
 enabled: boolean;
 approvalMode: 'required' | 'automatic';
 maxDepth: number;
 maxChildrenPerExecution: number;
 maxConcurrentChildren: number;
 maxDurationSeconds: number;
 maxOutputTokens: number;
 maxObjectiveBytes: number;
 maxResultBytes: number;
 maxTreeTokens: number;
}
export interface DelegationProfile { id: string; name: string; instructions: string; enabled: boolean; maxOutputTokens: number }
export interface DelegationPolicyView { policy: DelegationPolicy; revision: number; updatedBy?: string }
export interface DelegationProfileView { profile: DelegationProfile; revision: number; updatedBy?: string }
export const delegationLimits = [
 ['maxChildrenPerExecution', 'Children per parent', 32],
 ['maxConcurrentChildren', 'Concurrent children', 32],
 ['maxDurationSeconds', 'Duration including approval (seconds)', 600],
 ['maxTreeTokens', 'Tree token budget', 10000000],
 ['maxOutputTokens', 'Output token allowance', 4096],
 ['maxObjectiveBytes', 'Objective size (bytes)', 16384],
 ['maxResultBytes', 'Result size (bytes)', 65536],
] as const;
export function validDelegationPolicy(p: DelegationPolicy) {
 return p.maxDepth === 1 && ['required', 'automatic'].includes(p.approvalMode) && delegationLimits.every(([key,,max]) => Number.isInteger(p[key]) && p[key] >= 1 && p[key] <= max);
}
export function validDelegationProfile(p: DelegationProfile) {
 const bytes = (s: string) => new TextEncoder().encode(s).length;
 return /^[a-z0-9][a-z0-9-]{0,62}$/.test(p.id) && !!p.name.trim() && bytes(p.name) <= 120 && !!p.instructions.trim() && bytes(p.instructions) <= 4096 && !p.instructions.includes('\0') && Number.isInteger(p.maxOutputTokens) && p.maxOutputTokens >= 1 && p.maxOutputTokens <= 4096;
}
