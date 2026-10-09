import type {StagePlanInput,StageStateInput} from './productionMutations.mjs';
export interface StageHistoryEvent {sequence:number;operation_id:string;device_id:string;actor_id:string;kind:'configured'|'running'|'completed';reason:string;request:StagePlanInput|StageStateInput;result:{order_id:string;stage_id?:string;revision:number;status:string;repeated:boolean};created_at:string}
export function validStageHistoryEvent(v:unknown,orderID:string):boolean;
export function createProductionStageHistory(fetcher?:typeof fetch):{history(token:string,orderID:string,offset?:number):Promise<StageHistoryEvent[]>};
