import {createCatalogSearch} from '../local/catalogSearch.mjs';
import type {CatalogSearchProduct} from '../local/catalogSearch.mjs';
import type {Source89} from './types';
export async function catalog89(token:string,q:string,signal:AbortSignal):Promise<Source89<CatalogSearchProduct>>{const page=await createCatalogSearch()(token,{q,offset:0},signal);return {state:page.items.length?'ready':'empty',rows:page.items,at:new Date().toISOString(),partial:page.total>page.items.length};}
