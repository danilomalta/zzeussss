import type {ProductionOrder} from './commands.mjs';
export type ExecutionKind='reserve'|'materials'|'result'|'stage_plan'|'stage_state';
export interface ReserveInput{operation_id:string;reservation_id:string;order_id:string;reason:string}
export interface MaterialsInput{operation_id:string;reservation_id:string;action:'release'|'consume';reason:string}
export interface ResultInput{operation_id:string;result_id:string;order_id:string;expected_revision:number;produced_milli:number;reason:string}
export interface StageDefinition{stage_id:string;name:string;responsible_id:string}
export interface StagePlanInput{operation_id:string;order_id:string;stages:StageDefinition[];reason:string}
export interface StageStateInput{operation_id:string;order_id:string;stage_id:string;expected_revision:number;status:'running'|'completed';reason:string}
export type ExecutionBody=ReserveInput|MaterialsInput|ResultInput|StagePlanInput|StageStateInput;
export interface ExecutionIngredient{product_id:string;unit:string;planned_milli:number;reserved_milli:number;consumed_milli:number}
export interface ExecutionStage extends StageDefinition{position:number;status:'pending'|'running'|'completed';revision:number;updated_at:string}
export interface ProductionExecutionTrace{order:ProductionOrder;ingredients:ExecutionIngredient[];materials:{active_count:number;consumed_count:number;released_count:number;current:null|{id:string;order_id:string;location_id:string;status:'active'|'consumed';created_by:string;created_at:string;updated_at:string;items:{product_id:string;unit:string;quantity_milli:number}[]}};stages:null|{order_id:string;created_by:string;created_at:string;items:ExecutionStage[]};result:null|{result_id:string;order_id:string;reservation_id:string;product_id:string;location_id:string;unit:string;planned_milli:number;produced_milli:number;shortfall_milli:number;revision:number;status:'completed';repeated:false;operation_id:string;device_id:string;actor_id:string;reason:string;movement_id?:string;created_at:string}}
export const executionPaths:Record<ExecutionKind,string>;
export function executionInput(kind:string,input:unknown):ExecutionBody|null;
export function executionResult(kind:ExecutionKind,input:ExecutionBody,result:unknown):unknown;
export function productionExecutionTrace(value:unknown,order:ProductionOrder):ProductionExecutionTrace;
