import type {RecipeComparison} from './recipeComparison.mjs';
export function comparisonOutputCSV(value:RecipeComparison):string;
export function comparisonAddedCSV(value:RecipeComparison):string;
export function comparisonRemovedCSV(value:RecipeComparison):string;
export function comparisonChangedCSV(value:RecipeComparison):string;
export function comparisonUnchangedCSV(value:RecipeComparison):string;
