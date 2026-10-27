import type {ClosingSale92,Paper92} from './closing92.mjs';
import type {Review97} from './closingReview97.mjs';
export function serviceFor98<T>(selected:string,current:string,service:T):T|undefined;
export function reconciliationRows98(sales:ClosingSale92[],papers:Paper92[],complete:boolean,mode:'papers'|'integrated',w:Record<string,string>,money:(n:number)=>string):{review:Review97;rows:[string,string][]};
