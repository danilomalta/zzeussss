import {validExecutionTrace} from './productionExecution.mjs';
import {requireValid} from './operationsRead.mjs';
import {validProductionInput} from './productionMutations.mjs';
export function canConfigureStages(trace){return validExecutionTrace(trace,trace?.order?.id)&&['planned','approved'].includes(trace.order.status)&&trace.stages===null&&trace.materials.current===null;}
export function prepareStagePlan(trace,rows,reason,operationID,makeID){
 requireValid(canConfigureStages(trace));const input={operation_id:operationID,order_id:trace.order.id,stages:rows.map(r=>({stage_id:makeID(),name:r.name.trim(),responsible_id:r.responsible_id.trim()})),reason:reason.trim()};requireValid(validProductionInput('stage_plan',input));return input;
}
export function stageDecision(trace,actor){
 if(!validExecutionTrace(trace,trace?.order?.id)||trace.order.status!=='approved'||trace.materials.current?.status!=='consumed'||!trace.stages)return null;
 const s=trace.stages.items.find(i=>i.status!=='completed');if(!s||s.responsible_id!==actor)return null;
 return {stage_id:s.stage_id,expected_revision:s.revision,status:s.status==='pending'?'running':'completed'};
}
export function prepareStageState(trace,actor,reason,operationID){const choice=stageDecision(trace,actor);requireValid(!!choice);const input={operation_id:operationID,order_id:trace.order.id,...choice,reason:reason.trim()};requireValid(validProductionInput('stage_state',input));return input;}
