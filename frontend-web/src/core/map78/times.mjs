export function elapsedMinutes(start,end){
 if(typeof start!=='string'||typeof end!=='string')return null;
 const a=Date.parse(start),b=Date.parse(end);
 return Number.isFinite(a)&&Number.isFinite(b)&&b>=a?Math.floor((b-a)/60000):null;
}
export function deliveryTimes(row){
 return {waiting:elapsedMinutes(row.queued_at,row.picked_up_at),travel:elapsedMinutes(row.picked_up_at,row.arrived_at),atDestination:elapsedMinutes(row.arrived_at,row.completed_at),total:elapsedMinutes(row.queued_at,row.completed_at)};
}
export function locationState(position,now=Date.now(),staleMinutes=5){
 if(!position)return 'unknown';
 const timestamp=Date.parse(position.recorded_at);
 if(!Number.isFinite(timestamp)||timestamp>now+60000)return 'unknown';
 return now-timestamp>staleMinutes*60000?'stale':'recent';
}
