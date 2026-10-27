export interface Window114{from:string;to:string;as_of:string}
export interface Item114{key:string;payable_id:string;supplier_id:string;supplier_name:string;document_reference:string;state:'pending'|'approved'|'rejected'|'cancelled';installment_id:string;due_on:string;amount_cents:number;timing:'overdue'|'today'|'future';cash_applied_cents:number;outstanding_cents:number}
export interface Agenda114{window:Window114;summary:Record<string,number>;items:Item114[];currency:'BRL';raw:string}
export const header114:string[];export const metrics114:string[];
export function window114(v:unknown):Window114;
export function csvRows114(raw:unknown):string[][];
export function parseAgenda114(raw:string,query:Window114):Agenda114;
export function filterAgenda114(rows:Item114[],filter?:{query?:string;state?:string;timing?:string;openOnly?:boolean}):Item114[];
export function matrix114(rows:Item114[],words:Record<string,string>,window:Window114):(string|number)[][];
export function month114(today:string):Window114;
