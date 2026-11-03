import type {FleetRecord} from './source.mjs';
import type {LocalCapabilities} from '../local/localClient.mjs';
export function providerActions(record:FleetRecord,cap:LocalCapabilities):string[];
export function quoteCents(value:string):number;
export function quoteTotal(data:Record<string,unknown>):number;
