import {createContext,useContext,useEffect} from 'react';
import type {Area85,Workspace85} from './types';
import type {LocalCapabilities} from '../local/localClient.mjs';
export interface Context85{data:Workspace85;area:Area85;cap:LocalCapabilities;manager:boolean;locale:string;t:(key:string)=>string;money:(n:number)=>string;qty:(n:number)=>string;date:(s:string)=>string;edit:(run:(d:Workspace85)=>void)=>void;commit:(action:string,ref:string,run:(d:Workspace85)=>void,approval?:boolean)=>void;attempt:(run:()=>void)=>void;ask:(title:string,run:()=>void)=>void;notice:(key:string)=>void;setReport:(rows:(string|number)[][])=>void}
export const BusinessContext85=createContext<Context85|null>(null);
export function useBusiness85(){const c=useContext(BusinessContext85);if(!c)throw Error('context');return c;}
export function useForm85(name:string){const c=useBusiness85();return {get:(k:string,fallback='')=>c.data.forms[name]?.[k]??fallback,set:(k:string,v:string)=>c.attempt(()=>c.edit(d=>{(d.forms[name]??={})[k]=v;})),clear:()=>c.edit(d=>{delete d.forms[name];})};}
export function useReport85(rows:(string|number)[][]){const {setReport}=useBusiness85(),s=JSON.stringify(rows);useEffect(()=>{setReport(JSON.parse(s));return()=>setReport([]);},[s,setReport]);}
