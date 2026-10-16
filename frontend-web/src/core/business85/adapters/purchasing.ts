import type {Adapter85,Purchase85} from '../types';
import type {LocalCapabilities} from '../../local/localClient.mjs';
import {allowed85} from '../rules.mjs';
import {examples85} from '../data';
/** Endpoint intentionally unconfigured; samples require an explicit factory flag. */
export function purchasingAdapter85(cap:LocalCapabilities,demonstration=false):Adapter85<Purchase85>{return {async read(context,signal){if(signal.aborted)throw new DOMException('Aborted','AbortError');if(context.tenant_id!==cap.tenant_id||context.store_id!==cap.store_id||context.identity_id!==cap.identity_id||context.device_id!==cap.device_id||!allowed85(cap,'purchasing'))return {state:'forbidden',rows:[]};if(!demonstration)return {state:'unavailable',rows:[]};return {state:'ready',rows:examples85(JSON.stringify(context)).purchases,asOf:new Date().toISOString()};}};}
