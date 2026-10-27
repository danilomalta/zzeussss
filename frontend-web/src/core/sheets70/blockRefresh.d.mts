import type {Block} from './model.mjs';
export function blockRefreshSnapshot(block:Block):string;
export interface BlockRefreshTicket{signal:AbortSignal;block:Block;current(workbookID:string,live?:Block):boolean;finish():void}
export function createBlockRefreshGate():{begin(workbookID:string,block:Block):BlockRefreshTicket;invalidate(id?:string):void};
