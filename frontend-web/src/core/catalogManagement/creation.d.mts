import type {LocalSession,LocalCapabilities} from '../local/localClient.mjs';
import type {Product} from './model.mjs';
export function createCatalogCreator(options:{token:string;session:LocalSession;capabilities:LocalCapabilities;isCurrent:()=>boolean;send:(token:string,input:Product)=>Promise<unknown>}):{create(input:Product):Promise<unknown>};
