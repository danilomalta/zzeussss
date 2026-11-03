import type {MaterialCost,ScenarioInput,CostScenario,PlannedRecipe,SimultaneousPlan} from './planning.mjs';
import type {ProductionPeople} from './responsibles.mjs';
import type {ProductionStageEvent} from './stageHistory.mjs';
import type {RecipeStateInput,RecipeState} from './recipeState.mjs';
import type {AftermathKind,AftermathBody,ProductionAfterReference,ProductionDeclaration,ProductionDeclarationEvent,ProductionSummary,ProductionQuality,ProductionQualityReview} from './aftermath.mjs';
import type {ExecutionKind,ExecutionBody,ProductionExecutionTrace} from './execution.mjs';
import type {LocalSession,LocalCapabilities} from '../local/localClient.mjs';
import type {RecipeVersion,RecipeIngredient} from './source.mjs';
export type ProductionKind='recipe_state'|'recipe'|'order'|'state'|ExecutionKind|AftermathKind;
export interface PublishRecipe{operation_id:string;recipe_id:string;version_id:string;expected_revision:number;name:string;output_product_id:string;output_unit:string;yield_milli:number;ingredients:RecipeIngredient[]}
export interface PlanOrder{operation_id:string;order_id:string;version_id:string;location_id:string;responsible_id:string;planned_batches:number}
export interface ChangeOrder{operation_id:string;order_id:string;expected_revision:number;status:'approved'|'cancelled';reason:string}
export type ProductionBody=RecipeStateInput|PublishRecipe|PlanOrder|ChangeOrder|ExecutionBody|AftermathBody;
export interface ProductionCommand{version:1;scope:string;kind:ProductionKind;body:ProductionBody;uncertain:boolean}
export interface ProductionReceipt{kind:ProductionKind;input:ProductionBody;result:{recipe_id?:string;version_id?:string;order_id?:string;revision?:number;status?:string;repeated:boolean}}
export interface ProductionOrder{id:string;version_id:string;location_id:string;responsible_id:string;planned_batches:number;planned_output_milli:number;recipe:RecipeVersion;status:'planned'|'approved'|'cancelled'|'completed';revision:number;created_by:string;created_at:string;updated_at:string;completion_id?:string}
export interface ProductionEvent{operation_id:string;actor_id:string;kind:string;before_status:string;after_status:string;revision:number;reason:string;created_at:string}
export interface ProductionProduct{id:string;name:string;sku:string;unit:string}
export interface ProductionResolution{state:'confirmed'|'missing'|'discarded';receipt?:ProductionReceipt}
export interface ProductionCommands{scope:string;actor:string;canViewCosts:boolean;materialCost(version:RecipeVersion,batches:number,signal?:AbortSignal):Promise<MaterialCost>;orderMaterialCost(order:ProductionOrder,signal?:AbortSignal):Promise<MaterialCost>;costScenario(version:RecipeVersion,input:ScenarioInput,signal?:AbortSignal):Promise<CostScenario>;simultaneous(locationID:string,items:PlannedRecipe[],signal?:AbortSignal):Promise<SimultaneousPlan>;canListResponsibles:boolean;responsibles(signal?:AbortSignal):Promise<ProductionPeople>;canWrite:boolean;canLock:boolean;pending():ProductionCommand|null;prepare(kind:ProductionKind,body:ProductionBody):ProductionCommand;submit(command:ProductionCommand):Promise<ProductionResolution|null>;recover(send?:boolean,discard?:boolean):Promise<ProductionResolution|null>;recipeState(recipeID:string,signal?:AbortSignal):Promise<RecipeState>;declarations(kind:'loss'|'lot',reference:ProductionAfterReference,offset?:number,signal?:AbortSignal):Promise<ProductionSummary>;declaration(kind:'loss'|'lot',id:string,reference:ProductionAfterReference,signal?:AbortSignal):Promise<ProductionDeclaration>;declarationHistory(kind:'loss'|'lot',id:string,signal?:AbortSignal):Promise<ProductionDeclarationEvent[]>;quality(lot:ProductionDeclaration,signal?:AbortSignal):Promise<ProductionQuality>;qualityHistory(lot:ProductionDeclaration,offset?:number,signal?:AbortSignal):Promise<ProductionQualityReview[]>;stageHistory(orderID:string,offset?:number,signal?:AbortSignal):Promise<ProductionStageEvent[]>;trace(orderID:string,signal?:AbortSignal):Promise<ProductionExecutionTrace>;products(q?:string,offset?:number,signal?:AbortSignal):Promise<{items:ProductionProduct[];total:number}>;orders(offset?:number,signal?:AbortSignal):Promise<ProductionOrder[]>;order(id:string,signal?:AbortSignal):Promise<ProductionOrder>;history(id:string,offset?:number,signal?:AbortSignal):Promise<ProductionEvent[]>}
export function productionInput(kind:ProductionKind,input:unknown):ProductionBody;
export function productionReceipt(value:unknown,kind:ProductionKind,input:ProductionBody,original?:boolean):ProductionReceipt;
export function productionOrder(value:unknown):ProductionOrder;
export function parseProductionMilli(value:string):number;
export function createProductionCommands(options:{token:string;session:LocalSession;capabilities:LocalCapabilities;isCurrent:()=>boolean;storage:Storage;locks?:LockManager;fetcher?:typeof fetch}):ProductionCommands;
