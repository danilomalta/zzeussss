export interface Report76 {name:string;matrix:(string|number)[][];}
export function stageReport76(context:string,id:string,name:string,matrix:(string|number)[][],now?:number):void;
export function clearReport76():void;
export function takeReport76(context:string,id:string,now?:number):Report76|null;
export function reportBook76<T>(base:T,report:Report76):T;
