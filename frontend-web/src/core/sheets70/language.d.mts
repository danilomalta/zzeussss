export function sheetLanguageKey(language:string):'ptBR'|'enUS'|'esES'|'frFR'|'zhCN';
export function currentSheetLanguage<T extends {locale:string}>(getLanguage:()=>string,isCurrent:()=>boolean,load:(language:string)=>Promise<T>):Promise<T|null>;
