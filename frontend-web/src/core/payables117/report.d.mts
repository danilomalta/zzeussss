import type {Item114,Window114} from '../payables114/agenda.mjs';
export interface Supplier117{id:string;name:string;parts:number;documents:number;approved:number;pending:number;rejected:number;cancelled:number;applied:number;balance:number;overdue:number;nextDue:string|null;oldestDue:string|null}
export interface Report117{totals:Record<string,number>;buckets:Record<string,number>;suppliers:Supplier117[];parts:number;documents:number;window:Window114;currency:'BRL'}
export type Matrix117=(string|number)[][];
export interface Matrices117{summary:Matrix117;suppliers:Matrix117;items:Matrix117}
export const buckets117:string[];export const totals117:string[];
export function age117(due:string,reference:string):string;
export function summarize117(rows:Item114[],window:Window114):Report117;
export function sort117(rows:Supplier117[],field?:string,direction?:string,locale?:string):Supplier117[];
export function matrices117(report:Report117,rows:Item114[],words:Record<string,string>,meta:{readAt:string;scope:string}):Matrices117;
export function csv117(matrices:Matrices117,words:Record<string,string>):string;
export function sheet117(report:Report117,words:Record<string,string>):Matrix117;
