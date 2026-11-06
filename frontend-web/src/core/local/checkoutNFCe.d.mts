import type {CheckoutAdapter96} from './checkout96.mjs';
import type {LocalSession} from './localClient.mjs';
export interface NFCeSetup {nature:string;presence:string;policies:Record<number,string>;issuerAddress:Record<string,string>;unitPrices?:Record<number,number>}
export function createNFCeCheckoutAdapter(options:{getCurrent:()=>{token:string|null;session:LocalSession|null};getSetup:()=>NFCeSetup;fetcher?:typeof fetch;uuid?:()=>string;now?:()=>Date}):CheckoutAdapter96;
