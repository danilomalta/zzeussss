import {createContext,useContext} from 'react';
import type {Area89,Data89} from './types';
import type {LocalCapabilities} from '../local/localClient.mjs';
export interface Enterprise89{data:Data89;area:Area89;scope:string;token:string;cap:LocalCapabilities;w:Record<string,string>;locale:string;busy:boolean;dirty:(key:string,value:boolean)=>void;act:(action:string,name:string,change:(d:Data89)=>void)=>Promise<boolean>;error:(code:string)=>void;money:(n:number)=>string;date:(v:string)=>string;csv:(rows:(string|number)[][])=>void;sheets:(rows:(string|number)[][])=>void;refresh:()=>Promise<void>}
export const EnterpriseContext89=createContext<Enterprise89|null>(null);
export function useEnterprise89(){const c=useContext(EnterpriseContext89);if(!c)throw Error('context');return c;}
