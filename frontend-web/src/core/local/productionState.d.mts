import type {ProductionOrder} from './productionRead.mjs';
import type {StateInput} from './productionMutations.mjs';
export function transitionChoices(order:ProductionOrder):('approved'|'cancelled')[];
export function prepareProductionState(order:ProductionOrder,status:'approved'|'cancelled',reason:string,operationID:string):StateInput;
