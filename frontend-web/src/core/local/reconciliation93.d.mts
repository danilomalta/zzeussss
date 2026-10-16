import type {ClosingDraft92,ClosingSale92,Paper92} from './closing92.mjs';
export interface Customer93{document:string;club:'skip'|'member'|'join'}
export function validCustomer93(value:unknown):boolean;
export function partialCandidates93(p:Paper92,sales:ClosingSale92[]):ClosingSale92[];
export function assignSale93(d:ClosingDraft92,id:string,sale:string,sales:ClosingSale92[]):void;
export function reconcile93(sales:ClosingSale92[],papers:Paper92[]):{id:string;at:string;expected:number;cash:number;paper:number;uncovered:number;excess:number;payments:number;manual:boolean}[];
export function methodTotals93(sales:ClosingSale92[],papers:Paper92[],complete:boolean):{method:string;expected:number|null;documented:number;unassigned:number;pending:number|null;excess:number|null}[];
