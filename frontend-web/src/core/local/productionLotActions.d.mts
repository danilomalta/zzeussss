import type {LotSummary,ProductionLot} from './productionLots.mjs';
import type {QualityAudit} from './productionQuality.mjs';
import type {LotInput,VoidLotInput,QualityInput} from './productionMutations.mjs';
export function prepareLot(v:LotSummary,quantity:string,code:string,made:string,expiry:string,reason:string,operationID:string,lotID:string):LotInput;
export function prepareLotVoid(v:ProductionLot,reason:string,operationID:string):VoidLotInput;
export function prepareQuality(v:QualityAudit,status:'passed'|'failed',criterion:string,reason:string,operationID:string):QualityInput;
