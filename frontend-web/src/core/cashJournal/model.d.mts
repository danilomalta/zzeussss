import type {JournalPage} from './contracts';
export function journalID(v:unknown):v is string;
export function journalPage(v:unknown,cash:string,offset?:number,limit?:number,maximum?:number):JournalPage;
export function journalCSV(data:Uint8Array,cash:string):JournalPage;
export function journalMoney(value:number,locale?:string):string;
export const journalColumns:string[];
export function journalFail():never;
