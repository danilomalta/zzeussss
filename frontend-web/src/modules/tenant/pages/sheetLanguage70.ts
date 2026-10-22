import {sheetLanguageKey} from '../../../core/sheets70/language.mjs';
const loaders={ptBR:()=>import('./sheetLocales70/ptBR'),enUS:()=>import('./sheetLocales70/enUS'),esES:()=>import('./sheetLocales70/esES'),frFR:()=>import('./sheetLocales70/frFR'),zhCN:()=>import('./sheetLocales70/zhCN')};
export async function loadSheetLanguage(language:string){return (await loaders[sheetLanguageKey(language)]()).default;}
