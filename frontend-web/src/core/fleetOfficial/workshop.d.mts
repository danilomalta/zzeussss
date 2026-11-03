import type {LocalSession,LocalCapabilities} from '../local/localClient.mjs';
import type {FleetRecord} from './source.mjs';
import type {FleetBody} from './commands.mjs';
export interface CheckItem {id:string;label:string;required:boolean}
export interface CheckTemplate {name:string;items:CheckItem[];note:string}
export interface CheckAnswer {item_id:string;outcome:'pass'|'fail'|'not_applicable';note:string}
export type WorkshopResource='templates'|'requirements'|'inspections'|'labor';
export interface WorkshopReferences {request:FleetRecord|null;template:FleetRecord|null;binding:FleetRecord|null;epoch:number}
export interface WorkshopCommand {version:1;scope:string;resource:WorkshopResource;before:FleetRecord|null;references:WorkshopReferences;body:FleetBody;uncertain:boolean}
export interface WorkshopOutcome {state:'confirmed'|'missing'|'discarded';command:WorkshopCommand;receipt?:FleetRecord}
export interface WorkshopCommands {scope:string;readonly canWrite:boolean;assertCurrent():void;dispose():void;pending():WorkshopCommand|null;context(resource:WorkshopResource,requestID:string,templateID?:string):Promise<WorkshopReferences>;prepare(resource:WorkshopResource,before:FleetRecord|null,references:WorkshopReferences,body:FleetBody):WorkshopCommand;submit(value:WorkshopCommand):Promise<WorkshopOutcome|null>;recover(send?:boolean,discard?:boolean):Promise<WorkshopOutcome|null>}
export function createWorkshopCommands(options:{token:string;session:LocalSession;capabilities:LocalCapabilities;isCurrent:()=>boolean;storage:Pick<Storage,'getItem'|'setItem'|'removeItem'>;locks:LockManager;fetcher?:typeof fetch}):WorkshopCommands;
export function workshopActions(before:FleetRecord,request:FleetRecord|null,cap:LocalCapabilities):string[];
export function workshopTemplate(v:unknown):CheckTemplate;
export function workshopPassed(items:CheckItem[],answers:CheckAnswer[]):boolean;
export function workshopSeconds(start:string,end:string,now?:number):number;
