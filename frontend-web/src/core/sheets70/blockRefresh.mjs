function ordered(value){return Array.isArray(value)?value.map(ordered):value&&typeof value==='object'?Object.fromEntries(Object.keys(value).sort().map(key=>[key,ordered(value[key])])):value;}
export const blockRefreshSnapshot=block=>JSON.stringify(ordered(block));
// Separate tickets per block; cancellation is an optimization, identity is the guarantee.
export function createBlockRefreshGate(){
 const active=new Map();
 function invalidate(id){if(id===undefined){for(const ticket of active.values())ticket.controller.abort();active.clear();}else{active.get(id)?.controller.abort();active.delete(id);}}
 function begin(workbookID,block){invalidate(block.id);const controller=new AbortController(),snapshot=blockRefreshSnapshot(block),copy=structuredClone(block);const ticket={controller,signal:controller.signal,block:copy,current(book,live){return active.get(copy.id)===ticket&&!controller.signal.aborted&&book===workbookID&&!!live&&!live.detached&&blockRefreshSnapshot(live)===snapshot;},finish(){if(active.get(copy.id)===ticket)active.delete(copy.id);}};active.set(copy.id,ticket);return ticket;}
 return {begin,invalidate};
}
