export function createDraftRegistry(notify:(dirty:boolean)=>void):{set(id:string,value:boolean):void;has():boolean};
export function leavesDrafts(dirty:boolean,current:{pathname:string;search:string;hash:string},next:{pathname:string;search:string;hash:string}):boolean;
