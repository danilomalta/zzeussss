import type {Document113,Part113} from '../payables113/model.mjs';
export interface PayableEvent {operation_id:string;actor_id:string;action:'create'|'approve'|'reject'|'cancel';from_state:string;to_state:string;reason:string;created_at:string}
export interface PayableDocument extends Document113 {supplier_id:string;creator_id:string;description:string;issued_on:string;currency:'BRL';created_at:string;updated_at:string;last_operation_id:string;history:PayableEvent[];payment_execution_available:false;settled_cents:null}
export interface CreateInput {operation_id:string;payable_id:string;supplier_id:string;document_reference:string;description:string;issued_on:string;currency:'BRL';total_cents:number;installments:Part113[]}
export type DocumentAction='approve'|'reject'|'cancel';
export type DocumentCommand={version:1;kind:'create';document:string;body:CreateInput;expected:null;uncertain:boolean}|{version:1;kind:DocumentAction;document:string;body:{operation_id:string;reason:string};expected:{last_operation_id:string;state:string;creator_id:string};uncertain:boolean};
export interface DocumentReceipt {operation_id:string;repeated:boolean;payable:PayableDocument}
export interface DocumentPage {items:PayableDocument[];total_count:number;limit:20;offset:number;has_more:boolean}
export function centsInput(value:string):number;
export function createInput(value:CreateInput):CreateInput;
export function payableDocument(value:unknown):PayableDocument;
export function documentPage(value:unknown,offset:number):DocumentPage;
export function documentCommand(value:DocumentCommand):DocumentCommand;
export function availableActions(document:PayableDocument,actor:string):DocumentAction[];
export function documentReceipt(value:unknown,command:DocumentCommand,actor:string):DocumentReceipt;
