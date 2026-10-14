export interface Account{code:string;name:string;kind:'asset'|'liability'|'income'|'expense'|'equity'}
export interface Entry{id:string;date:string;description:string;lines:{account:string;side:'debit'|'credit';amount:number}[];reverses?:string}
export interface Finance{id:string;date:string;kind:'pay'|'receive';description:string;center:string;amount:number;status:'planned'|'checked'}
export interface WorkFile{format:'titan-workbench-73';scope:string;accounts:Account[];entries:Entry[];finance:Finance[];closed:string[]}
export function money(v:string):number;export function sum(v:number[]):number;export function validDate(v:string):boolean;
export function allocation(total:number,rows:{method:string;amount:number}[]):{paid:number;remaining:number;balanced:boolean};export function cashConference(rows:{method:string;amount:number}[]):Record<string,number>;
export function journal(e:Entry,a:Account[],closed?:string[]):Entry;export function reverse(e:Entry,id:string,date:string,a:Account[],closed:string[]):Entry;
export function balance(e:Entry[],a:Account[],from?:string,to?:string):(Account&{debit:number;credit:number;balance:number})[];
export function financeSummary(e:Finance[],from?:string,to?:string):{incoming:number;outgoing:number;net:number};
export function csv(r:unknown[][]):string;export function permitted(cap:unknown,area:string):boolean;export function validateFile(v:unknown,scope:string):WorkFile;export function xmlPreflight(text:string):string;
