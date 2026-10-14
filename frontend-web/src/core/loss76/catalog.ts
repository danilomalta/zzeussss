import {localClient,useLocalSession} from '../local/useLocalSession';
import type {LocalProduct} from '../local/localClient.mjs';
// Existing read-only endpoint. A page of products does not provide stock or expiry.
export async function catalogPage(token:string,offset:number):Promise<LocalProduct[]>{const rows=await localClient.products(token,offset);if(useLocalSession.getState().token!==token)throw Error('session');return rows;}
