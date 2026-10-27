import type {ExecutionBody} from './execution.mjs';
export interface ProductionStageEvent{sequence:number;operation_id:string;device_id:string;actor_id:string;kind:'configured'|'running'|'completed';reason:string;request:ExecutionBody;result:{order_id:string;stage_id?:string;revision:number;status:string;repeated:boolean};created_at:string}
export function productionStageHistory(value:unknown,orderID:string):ProductionStageEvent[];
