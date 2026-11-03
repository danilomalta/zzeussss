import type {Delivery,History,Origin,Result} from './contracts';
import type {DeliveryCommand} from './source.mjs';
export function deliveryActions(v:Delivery):string[];
export function cycleCommand(v:unknown,scope:string):DeliveryCommand;
export function cycleResult(v:unknown,c:DeliveryCommand,actor:string,device:string):Result;
export function deliveryHistory(v:unknown,id:string,offset?:number):History;
export function cycleOrigin(v:unknown,id:string):Origin;
export function deliveryFresh(c:DeliveryCommand,now?:number):DeliveryCommand;
