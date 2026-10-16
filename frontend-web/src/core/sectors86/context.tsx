import {createContext,useContext,useEffect} from 'react';
import type {Data86,Area86} from './types';
import type {LocalCapabilities} from '../local/localClient.mjs';
export interface Context86{data:Data86;area:Area86;cap:LocalCapabilities;manager:boolean;locale:string;t:(key:string)=>string;money:(n:number)=>string;qty:(n:number)=>string;date:(s:string)=>string;edit:(run:(d:Data86)=>void)=>void;commit:(action:string,ref:string,run:(d:Data86)=>void,approval?:boolean)=>void;attempt:(run:()=>void)=>void;ask:(title:string,run:()=>void)=>void;notice:(key:string)=>void;setReport:(rows:(string|number)[][])=>void}
export const SectorsContext86=createContext<Context86|null>(null);
export function useSectors86(){const c=useContext(SectorsContext86);if(!c)throw Error('context');return c;}
export function useForm86(name:string){const c=useSectors86();return {get:(k:string,fallback='')=>c.data.forms[name]?.[k]??fallback,set:(k:string,v:string)=>c.attempt(()=>c.edit(d=>{(d.forms[name]??={})[k]=v;})),clear:()=>c.edit(d=>{delete d.forms[name];})};}
export function useReport86(rows:(string|number)[][]){const {setReport}=useSectors86(),s=JSON.stringify(rows);useEffect(()=>{setReport(JSON.parse(s));return()=>setReport([]);},[s,setReport]);}
