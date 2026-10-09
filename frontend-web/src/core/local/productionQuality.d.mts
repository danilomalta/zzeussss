import type {ProductionLot} from './productionLots.mjs';
export interface QualityReview {lot_id:string;revision:number;operation_id:string;device_id:string;actor_id:string;status:'passed'|'failed';criterion:string;reason:string;lot_snapshot:ProductionLot;created_at:string}
export interface LotQuality {lot_id:string;lot_status:'recorded'|'voided';revision:number;status:'not_assessed'|'passed'|'failed';latest:QualityReview|null}
export interface QualityAudit {lot:ProductionLot;quality:LotQuality;items:QualityReview[];offset:number;has_more:boolean}
export function sameLotMetadata(a:unknown,b:unknown):boolean;
export function validQualityReview(v:unknown,id:string):boolean;
export function validLotQuality(v:unknown,id:string):boolean;
export function validQualityPage(items:unknown,id:string,offset:number):boolean;
export function validQualityAudit(lot:unknown,quality:unknown,items:unknown,offset:number):boolean;
export function createProductionQuality(fetcher?:typeof fetch):{audit(token:string,id:string,offset?:number):Promise<QualityAudit>};
