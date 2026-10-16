export interface Summary95{scope:string;mode:'live'|'demo';cash:string;expected:number;counted:number;difference:number;at:string}
export function validateSummary95(s:unknown,scope:string,mode:'live'|'demo'):Summary95;
