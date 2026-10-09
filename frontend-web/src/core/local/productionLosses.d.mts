import type {LossInput,VoidLossInput} from './productionMutations.mjs';
export interface ProductionLoss {id:string;result_id:string;product_id:string;unit:string;quantity_milli:number;reason:string;status:'recorded'|'voided';revision:number;created_by:string;created_at:string;updated_at:string}
export interface LossSummary {result_id:string;product_id:string;unit:string;planned_milli:number;produced_milli:number;shortfall_milli:number;recorded_loss_milli:number;unclassified_shortfall_milli:number;items:ProductionLoss[]}
export interface LossEvent {operation_id:string;device_id:string;actor_id:string;kind:'recorded'|'voided';revision:number;reason:string;created_at:string}
export function validLoss(v:unknown):boolean;
export function validLossSummary(v:unknown,id:string):boolean;
export function validLossHistory(v:unknown):boolean;
export function prepareLoss(v:LossSummary,value:string,reason:string,op:string,lossID:string):LossInput;
export function prepareVoidLoss(v:ProductionLoss,reason:string,op:string):VoidLossInput;
export interface LossAudit {loss:ProductionLoss;events:LossEvent[]}
export function validLossAudit(loss:unknown,events:unknown):boolean;
export function createProductionLosses(fetcher?:typeof fetch):{audit(token:string,id:string):Promise<LossAudit>;summary(token:string,id:string,offset?:number):Promise<LossSummary>;loss(token:string,id:string):Promise<ProductionLoss>;history(token:string,id:string):Promise<LossEvent[]>};
