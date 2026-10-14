import type {Decision} from './engine.mjs';
export interface LossCommands {previewDiscount(input:Decision):Promise<{preview_id:string;expected_version:number}>;applyDiscount(input:{preview_id:string;operation_id:string;expected_version:number}):Promise<{receipt_id:string}>;recordLoss(input:{product_id:string;quantity_milli:number;reason:string;operation_id:string;expected_version:number}):Promise<{receipt_id:string}>;}
export const commandsAdapter:LossCommands|null=null;
