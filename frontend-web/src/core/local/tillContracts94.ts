import type {Part94,Request94,Bag94} from './till94.mjs';
// Contracts for a future authorized gateway; no guessed URLs and no production mock.
export interface Applied94{operation_id:string;server_revision:number;state:'pending'|'confirmed'|'rejected';reference:string}
export interface TillGateway94{
 payments:{create:(sale:string,parts:Part94[],operation_id:string)=>Promise<Applied94>;status:(operation_id:string)=>Promise<Applied94>};
 movements:{submit:(request:Request94,operation_id:string)=>Promise<Applied94>};
 approvals:{review:(request:string,decision:'approve'|'deny',reason:string,expected_revision:number)=>Promise<Applied94>};
 returns:{submit:(request:Request94,operation_id:string)=>Promise<Applied94>};
 bags:{submit:(bag:Bag94,expected_revision:number)=>Promise<Applied94>};
 journal:{read:(cash:string,cursor?:string)=>Promise<{opening:number;withdrawals:number;deposits:number;refunds:number;expected:number;complete:boolean;revision:number}>};
}
// Binding is deliberately absent until the server exposes and validates these contracts.
export const tillGateway94:TillGateway94|null=null;
