import {createContext,useContext} from 'react';
import type {Data90,Area90} from './types';
import type {LocalCapabilities} from '../local/localClient.mjs';
export interface Operation90{data:Data90;area:Area90;cap:LocalCapabilities;scope:string;token:string;w:Record<string,string>;locale:string;busy:boolean;dirty:(id:string,state:boolean)=>void;act:(action:string,reference:string,change:(d:Data90)=>void)=>Promise<boolean>;error:(s:string)=>void;money:(v:number)=>string;quantity:(v:number)=>string;date:(v:string)=>string;csv:(rows:(string|number)[][])=>void;sheets:(rows:(string|number)[][])=>void}
export const OperationContext90=createContext<Operation90|null>(null);
export function useOperation90(){const value=useContext(OperationContext90);if(!value)throw Error('context');return value;}
