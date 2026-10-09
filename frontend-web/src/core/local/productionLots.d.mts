export interface ProductionLot {id:string;result_id:string;product_id:string;unit:string;quantity_milli:number;code:string;manufactured_on:string;expires_on:string;reason:string;status:'recorded'|'voided';revision:number;created_by:string;created_at:string;updated_at:string}
export interface LotSummary {result_id:string;product_id:string;unit:string;produced_milli:number;assigned_milli:number;unassigned_milli:number;items:ProductionLot[]}
export function calendarDate(v:unknown):boolean;
export function recordedTime(v:unknown):boolean;
export function reasonText(v:unknown):boolean;
export function validLot(v:unknown):boolean;
export function validLotSummary(v:unknown,id:string):boolean;
export function createProductionLots(fetcher?:typeof fetch):{audit(token:string,id:string):Promise<LotAudit>;summary(token:string,id:string,offset?:number):Promise<LotSummary>;lot(token:string,id:string):Promise<ProductionLot>};
export interface LotEvent {operation_id:string;device_id:string;actor_id:string;kind:'recorded'|'voided';revision:number;reason:string;created_at:string}
export interface LotAudit {lot:ProductionLot;events:LotEvent[]}
export function validLotEvents(v:unknown):boolean;
export function validLotAudit(lot:unknown,events:unknown):boolean;
