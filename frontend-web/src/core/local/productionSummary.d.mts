import type {ProductionOrder} from './productionRead.mjs';
export interface PlannedGroup {status:string;location_id:string;product_id:string;unit:string;quantity_milli:string;order_ids:string[];version_ids:string[]}
export function plannedOrderSummary(items:ProductionOrder[],kind:'outputs'|'ingredients'):PlannedGroup[];
