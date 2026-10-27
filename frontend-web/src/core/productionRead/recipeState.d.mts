export interface RecipeStateInput{operation_id:string;recipe_id:string;expected_revision:number;status:'active'|'inactive';reason:string}
export interface RecipeState{recipe_id:string;revision:number;status:'active'|'inactive';repeated:boolean}
export function recipeStateInput(value:unknown):RecipeStateInput;
export function productionRecipeState(value:unknown,recipeID:string):RecipeState;
export function recipeStateResult(value:unknown,input:RecipeStateInput):RecipeState;
