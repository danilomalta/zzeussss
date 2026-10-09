import type {ProductionOrder} from './productionRead.mjs';
import type {ResultInput} from './productionMutations.mjs';
export interface ExecutionTrace {order:ProductionOrder;ingredients:{product_id:string;unit:string;planned_milli:number;reserved_milli:number;consumed_milli:number}[];materials:{active_count:number;consumed_count:number;released_count:number;current:{id:string;order_id:string;location_id:string;status:string;created_by:string;created_at:string;updated_at:string;items:{product_id:string;unit:string;quantity_milli:number}[]}|null};stages:{order_id:string;items:{stage_id:string;name:string;responsible_id:string;position:number;status:string;revision:number}[]}|null;result:{result_id:string;order_id:string;product_id:string;location_id:string;unit:string;planned_milli:number;produced_milli:number;shortfall_milli:number;revision:number;status:string;reason:string;created_at:string}|null}
export function validExecutionTrace(v:unknown,id:string):boolean;
export function executionChoices(v:ExecutionTrace):string[];
export function completionReady(v:ExecutionTrace):boolean;
export function prepareCompletion(v:ExecutionTrace,value:string,reason:string,op:string,resultID:string):ResultInput;
export function createProductionExecution(fetcher?:typeof fetch):{trace(token:string,id:string):Promise<ExecutionTrace>};
