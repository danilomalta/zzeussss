export function elapsedMinutes(start:string|null,end:string|null):number|null;
export function deliveryTimes(row:{queued_at:string|null;picked_up_at:string|null;arrived_at:string|null;completed_at:string|null}):{waiting:number|null;travel:number|null;atDestination:number|null;total:number|null};
export function locationState(position:{recorded_at:string}|null,now?:number,staleMinutes?:number):'unknown'|'stale'|'recent';
