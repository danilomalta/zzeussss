export interface MovementBody{session_id:string;operation_id:string;kind:'withdrawal'|'supply';amount_cents:number;reason:string}
export interface MovementCommand{version:1;scope:string;body:MovementBody;uncertain:boolean}
export interface MovementReceipt extends MovementBody{movement_id:string;created_at:string;repeated:false}
export interface CashMovement{key:string;pending():MovementCommand|null;assertCurrent():void;readonly canWrite:boolean;prepare(v:MovementBody):MovementCommand;submit(c:MovementCommand):Promise<MovementReceipt|null>;recover(send?:boolean,discard?:boolean):Promise<MovementReceipt|null>;dispose():void}
export function createCashMovement(input:{token:string;session:import('./localClient.mjs').LocalSession;capabilities:import('./localClient.mjs').LocalCapabilities;isCurrent:()=>boolean;storage:Pick<Storage,'getItem'|'setItem'|'removeItem'>;locks:LockManager;fetcher?:typeof fetch}):CashMovement;
