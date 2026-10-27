export function sheetLanguageKey(language){const prefix=String(language||'').split('-')[0];return {pt:'ptBR',en:'enUS',es:'esES',fr:'frFR',zh:'zhCN'}[prefix]||'ptBR';}
// Imports may finish after navigation or a second language choice. A caller
// must never mount an editor or change its locale using that obsolete result.
export async function currentSheetLanguage(getLanguage,isCurrent,load){
 while(isCurrent()){
  const pack=await load(getLanguage());
  if(!isCurrent())return null;
  if(pack.locale===sheetLanguageKey(getLanguage()))return pack;
 }
 return null;
}
