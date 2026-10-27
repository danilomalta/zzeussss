// Tracks only in-memory forms. Never reads, clears or resends the operation queue.
export function createDraftRegistry(notify){
 const dirty=new Set(),selection=new Set();
 return {set(id,value,warnOnSelection=true){const before=dirty.size>0;if(value){dirty.add(id);if(warnOnSelection)selection.add(id);else selection.delete(id);}else{dirty.delete(id);selection.delete(id);}const after=dirty.size>0;if(before!==after)notify(after);},has(){return dirty.size>0;},hasSelection(){return selection.size>0;}};
}
export function leavesDrafts(dirty,current,next){return Boolean(dirty&&(current.pathname!==next.pathname||current.search!==next.search||current.hash!==next.hash));}
