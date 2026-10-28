import type {ProductionCommand} from './commands.mjs';
export function watchProductionQueue(options:{
 source:{scope:string;pending():ProductionCommand|null};
 target:EventTarget;
 visibility?:Document;
 onChange:(value:ProductionCommand|null)=>void;
 onError:()=>void;
}):()=>void;
