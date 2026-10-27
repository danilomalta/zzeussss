export const fleetResources:Readonly<Record<string,readonly [string,string]>>;
export const financialResources:readonly string[];
export function fleetID(v:unknown):boolean;
export function fleetStamp(v:unknown):boolean;
export function fleetPeriod(input:unknown):{from:string;to:string};
export function fleetDecimal(v:string,scale?:number,locale?:string):string;
