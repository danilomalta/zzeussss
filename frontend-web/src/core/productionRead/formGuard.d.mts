export function createDraftRegistry(notify:(dirty:boolean)=>void):{set(id:string,value:boolean,warnOnSelection?:boolean):void;has():boolean;hasSelection():boolean};
export function leavesDrafts(dirty:boolean,current:{pathname:string;search:string;hash:string},next:{pathname:string;search:string;hash:string}):boolean;
