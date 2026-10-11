import type {ExecutionTrace} from './productionExecution.mjs';
import type {ProductionLot} from './productionLots.mjs';
import type {LotQuality} from './productionQuality.mjs';
export interface ProductionTrace extends ExecutionTrace {losses:{recorded_milli:number;unclassified_milli:number}|null;lots:{produced_milli:number;assigned_milli:number;unassigned_milli:number;total_count:number;offset:number;has_more:boolean;items:{lot:ProductionLot;quality:LotQuality}[]}|null}
export function validProductionTrace(value:unknown,orderID:string,offset?:number):boolean;
export function createProductionTrace(fetcher?:typeof fetch):{trace(token:string,orderID:string,offset?:number):Promise<ProductionTrace>};
