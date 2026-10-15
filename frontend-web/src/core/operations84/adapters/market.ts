import type {SourceAdapter84,Offer84} from '../types';
import {allowed84} from '../rules.mjs';
import {examples84} from '../data';
/** No backend endpoint is assumed. Examples require an explicit factory flag. */
export function marketAdapter84(demonstration=false):SourceAdapter84<Offer84>{return {async read(context,cap,signal){if(signal.aborted)throw new DOMException('Aborted','AbortError');if(context.tenant_id!==cap.tenant_id||context.store_id!==cap.store_id||context.identity_id!==cap.identity_id||context.device_id!==cap.device_id||!allowed84(cap,'opportunities'))return {state:'forbidden',rows:[]};if(!demonstration)return {state:'unavailable',rows:[]};return {state:'ready',rows:examples84(JSON.stringify(context)).offers,asOf:new Date().toISOString()};}};}
