import type {LocalCapabilities} from '../local/localClient.mjs';
export type RequestStatus='open'|'triaged'|'approved'|'in_service'|'waiting_parts'|'inspection'|'released'|'cancelled';
export interface Actor{id:string;cap:LocalCapabilities}
export interface Equipment{id:string;name:string;reference:string;kind:'machine'|'forklift'|'other';location:string;meter:number;nextService:number}
export interface RequestInput{id:string;assetId:string;issue:string;location:string;priority:string;blocks:boolean;impact:string;delivery:string}
export interface MaintenanceRequest extends RequestInput{requester:string;assigned:string;status:RequestStatus;revision:number;createdAt:string;updatedAt:string;history:{revision:number;actor:string;note:string;status:RequestStatus;at:string}[]}
export interface CostInput{id:string;assetId:string;kind:string;amount:number;note:string}
export interface ProfileInput{id:string;acquisition:number;residual:number;months:number;elapsed:number;revenue:number;distance:number;hours:number;downtime:number;estimatedLoss:number}
export interface MaintenanceState{version:1;equipment:Equipment[];requests:MaintenanceRequest[];costs:(CostInput&{actor:string;at:string})[];profiles:(ProfileInput&{at:string})[]}
export const statuses:RequestStatus[];export const priorities:string[];export const costKinds:string[];
export function maintenanceAccess(cap:LocalCapabilities|null|undefined):boolean;
export function cents(value:string):number;export function exactSum(values:number[]):number;
export function emptyMaintenance():MaintenanceState;export function validMaintenance(value:unknown):value is MaintenanceState;
export function addEquipment(state:MaintenanceState,input:Equipment,actor:Actor):MaintenanceState;
export function openRequest(state:MaintenanceState,input:RequestInput,actor:Actor,assetIds:string[],at:string):MaintenanceState;
export function changeRequest(state:MaintenanceState,id:string,expected:number,action:string,note:string,actor:Actor,at:string):MaintenanceState;
export function addCost(state:MaintenanceState,input:CostInput,actor:Actor,assetIds:string[],at:string):MaintenanceState;
export function setProfile(state:MaintenanceState,input:ProfileInput,actor:Actor,assetIds:string[],at:string):MaintenanceState;
export function assetStatus(state:MaintenanceState,id:string,base?:string,running?:boolean):string;
export function costSummary(state:MaintenanceState,id:string,legacy?:number[]):{operating:number;profile:null|(ProfileInput&{at:string;depreciation:number;bookEstimate:number;contribution:number;costPerKm:null|{numerator:number;denominator:number};costPerHour:null|{numerator:number;denominator:number}})};
