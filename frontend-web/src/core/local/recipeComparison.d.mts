import type {RecipeVersion} from './productionRead.mjs';
export type Difference={direction:'increase'|'decrease'|'same'|'incompatible';numerator:string|null;denominator:string|null;unit:string};
type Ingredient=RecipeVersion['ingredients'][number];
export type RecipeComparison={before:RecipeVersion;after:RecipeVersion;same_product:boolean;yield_difference:Difference|null;added:Ingredient[];removed:Ingredient[];changed:{product_id:string;before:Ingredient;after:Ingredient;difference:Difference}[];unchanged:string[]};
export function validComparableVersion(v:unknown):boolean;
export function quantityDifference(before:{quantity_milli:number;unit:string},after:{quantity_milli:number;unit:string}):Difference;
export function compareRecipeVersions(before:RecipeVersion,after:RecipeVersion):RecipeComparison;
export function createRecipeComparison(fetcher?:typeof fetch):{compare(token:string,from:string,to:string):Promise<RecipeComparison>};
