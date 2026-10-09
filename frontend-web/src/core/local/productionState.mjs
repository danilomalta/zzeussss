import {validProductionOrder} from './productionRead.mjs';
import {validProductionInput} from './productionMutations.mjs';
import {LocalAPIError} from './localClient.mjs';
export function transitionChoices(order){
 if(!validProductionOrder(order) || order.revision>=2147483647)return [];
 if(order.status==='planned')return ['approved','cancelled'];
 if(order.status==='approved')return ['cancelled'];return [];
}
export function prepareProductionState(order,status,reason,operationID){
 const input={operation_id:operationID,order_id:order?.id,expected_revision:order?.revision,status,reason:reason.trim()};
 if(!transitionChoices(order).includes(status)||!validProductionInput('state',input))throw new LocalAPIError(0,'Confira o estado consultado, a revisão e o motivo. Consulte novamente se a ordem mudou.');
 return input;
}
