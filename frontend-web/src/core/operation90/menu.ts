import {areas90,can90} from './engine.mjs';
import {words90} from './words';
import type {LocalCapabilities} from '../local/localClient.mjs';
interface Card90{id:string;path:string;icon:string;label:string;group:string;planned:boolean}
export function menu90(existing:Card90[],planned:Card90[],cap:LocalCapabilities|null,locale:string){const w=words90(locale),map:Record<string,string>={'payment-tools':'pos',cash:'closing',fiscal:'fiscal',inventory:'inventory',purchasing:'purchases',discounts:'losses'};for(const card of [...existing,...planned]){const area=map[card.id];if(area&&can90(cap,area)){card.path='/local/operation?area='+area;card.label=w[area];card.planned=false;}}
 const first=areas90.find(a=>can90(cap,a));if(first&&!existing.some(c=>c.id==='operation90'))existing.push({id:'operation90',path:'/local/operation?area='+first,icon:'▦',label:w.operationCenter,group:'operation',planned:false});
}
