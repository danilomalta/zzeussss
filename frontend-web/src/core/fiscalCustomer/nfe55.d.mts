export class NFe55Error extends Error{code:string}
export interface NFe55Status{state:string;message?:string;error_code?:string;key?:string;intent_id?:string}
export interface NFe55Preview{sale_id:string;intent_id:string;total_cents:number;items:{product_id:string;quantity_milli:number;unit_price_cents:number;total_cents:number}[];mapping_supported:boolean;status:NFe55Status}
export function createNFe55Client(options:{token:string;isCurrent:()=>boolean;fetcher?:typeof fetch}):{preview:(sale:string)=>Promise<NFe55Preview>;emit:(sale:string,input:unknown)=>Promise<NFe55Status>;query:(sale:string)=>Promise<NFe55Status>;cancel:(sale:string,input:unknown)=>Promise<NFe55Status>;danfe:(sale:string)=>Promise<Blob>};
