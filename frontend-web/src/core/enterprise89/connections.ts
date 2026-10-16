import type {Source89} from './types';
export interface ConnectionStatus89{id:string;name:string;state:'connected'|'failed'|'disabled';checkedAt:string}
// No connector status endpoint exists. A local setup draft is never a live connection.
export async function connections89():Promise<Source89<ConnectionStatus89>>{return {state:'unavailable',rows:[]};}
