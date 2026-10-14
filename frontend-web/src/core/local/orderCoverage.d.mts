import type {ExecutionTrace} from './productionExecution.mjs';
import type {Alternatives} from './catalogPlanning.mjs';
import type {AlternativePlan} from './alternativePlan.mjs';
export interface OrderCoverage {source:{trace:ExecutionTrace;capacity:Alternatives};order:ExecutionTrace['order'];released_reservations:number;missing_batches:number;plan:AlternativePlan}
export function orderCoverage(trace:ExecutionTrace,snapshot:Alternatives):OrderCoverage;
export function createOrderCoverage(fetcher?:typeof fetch):{coverage(token:string,orderID:string):Promise<OrderCoverage>};
