import type {Lot} from './engine.mjs';
export interface LotsAdapter {read(context:{tenant_id:string;store_id:string},signal:AbortSignal):Promise<{rows:Lot[];as_of:string;complete:boolean}>;}
// No connected source is installed until the backend supplies authorized lots and demand.
export const lotsAdapter:LotsAdapter|null=null;
