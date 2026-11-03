import type {LocalSession,LocalCapabilities} from '../local/localClient.mjs';
export interface FleetRecord {id:string;kind:string;state:string;revision:number;asset_id?:string;request_id?:string;assigned_to?:string;created_by?:string;created_at?:string;updated_at?:string;actor_id?:string;device_id?:string;recorded_at?:string;data:Record<string,unknown>}
export interface FleetPage {items:FleetRecord[];total:number;limit:number;offset:number;measured_at?:string}
export interface FleetEvent {action:string;reason:string;actor_id?:string;device_id?:string;result:FleetRecord & {record?:FleetRecord;operation_id:string}}
export interface FleetHistory {items:FleetEvent[];total:number;limit:number;offset:number}
export interface FleetFilters {asset_id?:string;state?:string;assigned_to?:string;request_id?:string}
export interface FleetPeriod {from:string;to:string}
export interface FleetNotice {id:number;request_id:string;revision:number;recipient_id:string;created_at:string;acknowledged:boolean}
export interface FleetNoticePage {items:FleetNotice[];next_cursor:number;limit:number;push_delivered:boolean}
export interface FleetPreventivePage {items:{plan:FleetRecord;due_by_date:boolean;due_by_meter:boolean}[];total:number;offset:number;limit:number;measured_at:string;automatic_request:boolean}
export interface FleetRead {notices(after?:number,signal?:AbortSignal):Promise<FleetNoticePage>;preventive(id:string,offset?:number,signal?:AbortSignal):Promise<FleetPreventivePage>;scope:string;assertCurrent():void;dispose():void;list(name:string,filters?:FleetFilters,offset?:number,signal?:AbortSignal):Promise<FleetPage>;record(name:string,id:string,signal?:AbortSignal):Promise<FleetRecord>;history(name:string,id:string,offset?:number,signal?:AbortSignal):Promise<FleetHistory>;report(kind:'availability'|'workshop'|'costs',id:string,period?:FleetPeriod,signal?:AbortSignal):Promise<Record<string,unknown>>}
export function createFleetRead(options:{token:string;session:LocalSession;capabilities:LocalCapabilities;isCurrent:()=>boolean;fetcher?:typeof fetch}):FleetRead;
