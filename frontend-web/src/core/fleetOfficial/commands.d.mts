import type {LocalSession,LocalCapabilities} from '../local/localClient.mjs';
import type {FleetRecord} from './source.mjs';
export interface FleetBody {operation_id:string;id:string;expected_revision:number;action:string;reason:string;data:Record<string,unknown>}
export interface FleetCommand {version:1;scope:string;resource:'assets'|'requests'|'providers'|'quotes';before:FleetRecord|null;body:FleetBody;uncertain:boolean;quoteRequest?:FleetRecord}
export interface FleetCommandResult {state:'confirmed'|'missing'|'discarded';command:FleetCommand;receipt?:{operation_id:string;record:FleetRecord;repeated:boolean}}
export interface FleetCommands {scope:string;readonly canWrite:boolean;assertCurrent():void;dispose():void;pending():FleetCommand|null;quoteContext(requestID:string):Promise<FleetRecord>;prepare(resource:'assets'|'requests'|'providers'|'quotes',before:FleetRecord|null,body:FleetBody,quoteRequest?:FleetRecord):FleetCommand;submit(value:FleetCommand):Promise<FleetCommandResult|null>;recover(send?:boolean,discard?:boolean):Promise<FleetCommandResult|null>}
export function createFleetCommands(options:{token:string;session:LocalSession;capabilities:LocalCapabilities;isCurrent:()=>boolean;storage:Pick<Storage,'getItem'|'setItem'|'removeItem'>;locks:LockManager;fetcher?:typeof fetch}):FleetCommands;
export function requestActions(record:FleetRecord,cap:LocalCapabilities):string[];
export function fleetEqual(a:unknown,b:unknown):boolean;
