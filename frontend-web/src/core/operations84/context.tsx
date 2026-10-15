import {createContext,useContext,useEffect} from 'react';
import type {Area84,Workspace84} from './types';
import type {LocalCapabilities} from '../local/localClient.mjs';
export interface Ops84 {
 data:Workspace84;area:Area84;locale:string;cap:LocalCapabilities;manager:boolean;readOnly:boolean;
 t:(key:string)=>string;money:(n:number)=>string;qty:(n:number)=>string;date:(s:string)=>string;
 edit:(mutate:(draft:Workspace84)=>void)=>void;
 commit:(action:string,reference:string,mutate:(draft:Workspace84)=>void,approval?:boolean)=>void;
 attempt:(run:()=>void)=>void;ask:(title:string,run:()=>void)=>void;notice:(key:string)=>void;
 setReport:(matrix:(string|number)[][])=>void;
}
export const OperationsContext84=createContext<Ops84|null>(null);
export function useOps84(){const c=useContext(OperationsContext84);if(!c)throw Error('context');return c;}
export function useForm84(name:string){const {data,edit,readOnly,attempt}=useOps84();return {g:(key:string,fallback='')=>data.forms[name]?.[key]??fallback,s:(key:string,value:string)=>{if(!readOnly)attempt(()=>edit(d=>{(d.forms[name]??={})[key]=value;}));},clear:()=>{if(!readOnly)attempt(()=>edit(d=>{delete d.forms[name];}));}};}
export function useReport84(matrix:(string|number)[][]){const {setReport}=useOps84();const serialized=JSON.stringify(matrix);useEffect(()=>{setReport(JSON.parse(serialized));return()=>setReport([]);},[serialized,setReport]);}
