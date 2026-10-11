import type {ReceivingHistory,HistoryEntry} from './receivingRead.mjs';
export interface ReceivingEventFilters {kind?:string;delivery_reference?:string;location_id?:string;actor_id?:string;reason?:string}
export interface FilteredReceivingEvents {filters:ReceivingEventFilters;order_id:string;offset:number;total_events:number;page_events:number;matched_events:number;items:HistoryEntry[]}
export function receivingEventFilters(value?:ReceivingEventFilters):ReceivingEventFilters;
export function filterReceivingEvents(history:ReceivingHistory,value?:ReceivingEventFilters):FilteredReceivingEvents;
