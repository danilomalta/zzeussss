import {read86} from '../sectors86/storage';
import type {Data86} from '../sectors86/types';
import type {Source89} from './types';
export async function production89(scope:string):Promise<Source89<Data86>>{const data=await read86(scope);return {state:data?'ready':'empty',rows:data?[data]:[],at:new Date().toISOString()};}
