import type {ExecutionTrace} from './productionExecution.mjs';
import type {StagePlanInput,StageStateInput} from './productionMutations.mjs';
export function canConfigureStages(v:ExecutionTrace):boolean;
export function prepareStagePlan(v:ExecutionTrace,rows:{name:string;responsible_id:string}[],reason:string,op:string,makeID:()=>string):StagePlanInput;
export function stageDecision(v:ExecutionTrace,actor:string):{stage_id:string;expected_revision:number;status:'running'|'completed'}|null;
export function prepareStageState(v:ExecutionTrace,actor:string,reason:string,op:string):StageStateInput;
