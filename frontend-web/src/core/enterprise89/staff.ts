import {localClient} from '../local/useLocalSession';
import type {Source89} from './types';
export interface Staff89{id:string;name:string;role:string;status:string}
export async function staff89(token:string,signal:AbortSignal):Promise<Source89<Staff89>>{const rows=await localClient.staff(token);if(signal.aborted)throw Error('aborted');return {state:rows.length?'ready':'empty',rows:rows.map(r=>({id:r.identity_id,name:r.name,role:r.role,status:r.status})),at:new Date().toISOString()};}
