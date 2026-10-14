export interface Context74{tenant_id:string;store_id:string;identity_id:string;device_id:string}
export function contextOf(session:unknown,capabilities:unknown,now?:number):Readonly<Context74>;
export function contextKey(ctx:Context74):string;export function sameContext(a:unknown,b:unknown):boolean;
export function assertRecord<T>(r:T,ctx:Context74,scope?:'store'|'company'):T;
export function operational(e:unknown):'operational'|'unverified';
export function eventEnvelope(ctx:Context74,input:{event_id:string;operation_id:string;type:string;version:number;occurred_at:string}):Readonly<Context74&{event_id:string;operation_id:string;type:string;version:number;occurred_at:string}>;
