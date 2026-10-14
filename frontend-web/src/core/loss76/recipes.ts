import type {Recipe} from './engine.mjs';
export interface RecipesAdapter {read(signal:AbortSignal):Promise<{id:string;name:string;unit:string;recipe:Recipe}[]>;}
export const recipesAdapter:RecipesAdapter|null=null;
