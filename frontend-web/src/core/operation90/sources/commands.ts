import type {Area90} from '../types';
/** Register implementations only after an actual, documented backend contract exists. */
export interface Command90{area:Area90;operationId:string;expectedRevision:number;payload:unknown}
export interface CommandResult90{state:'confirmed'|'pending'|'rejected';operationId:string;reference:string}
export interface CommandAdapter90{execute(input:Command90,signal:AbortSignal):Promise<CommandResult90>}
export function commandAdapter90():CommandAdapter90|null{return null;}
