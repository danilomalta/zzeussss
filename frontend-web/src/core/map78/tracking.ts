export type DeliveryStatus='waiting'|'traveling'|'arrived'|'completed';
export interface CourierPosition {
 latitude:number;longitude:number;accuracy_m:number;recorded_at:string;
}
export interface Delivery {
 id:string;courier_id:string;reference:string;status:DeliveryStatus;
 queued_at:string|null;picked_up_at:string|null;arrived_at:string|null;completed_at:string|null;
}
export interface Courier {
 id:string;name:string;state:string|null;position:CourierPosition|null;
}
export interface DeliveryContext {tenant_id:string;store_id:string;}
export interface DeliverySnapshot {
 context:DeliveryContext;received_at:string;couriers:Courier[];deliveries:Delivery[];
}
export type TrackingResult={state:'unavailable'}|{state:'ready';snapshot:DeliverySnapshot};
// Backend integration point. No endpoint has been implemented or assumed here.
export async function readTracking(_context:DeliveryContext,signal:AbortSignal):Promise<TrackingResult>{
 if(signal.aborted)throw new DOMException('Aborted','AbortError');
 return {state:'unavailable'};
}
