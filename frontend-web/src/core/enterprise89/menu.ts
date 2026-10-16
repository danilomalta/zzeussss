import {can89} from './engine.mjs';
import {words89} from './words';
import type {LocalCapabilities} from '../local/localClient.mjs';
import type {Profile86} from '../sectors86/types';
import type {Area89} from './types';
interface Card89{id:string;path:string;icon:string;label:string;group:string;planned:boolean}
export function menu89(existing:Card89[],planned:Card89[],cap:LocalCapabilities|null,profile:Profile86|null,locale:string){
 const w=words89(locale),mapped:Record<string,Area89>={'payment-tools':'pdv',finance:'finance',fiscal:'fiscal',integrations:'connections',staff:'access',production:'pcp'};
 for(const item of [...existing,...planned]){const area=mapped[item.id];if(!area)continue;if(area==='pcp'&&profile?.configured&&!profile.production){item.group='restricted89';continue;}if(can89(cap,area)){item.path='/local/enterprise?area='+area;item.label=w[area];item.planned=false;}else item.group='restricted89';}
 const added:[Area89,string,string][]=[['inventory','operation','▦'],['crm','management','♡'],['channels','operation','⇄'],['branches','administration','▥'],['tasks','management','✓'],['privacy','administration','◈']];
 for(const [area,group,icon]of added)if(can89(cap,area)&&![...existing,...planned].some(i=>i.id===area))existing.push({id:area,path:'/local/enterprise?area='+area,icon,label:w[area],group,planned:false});
}
