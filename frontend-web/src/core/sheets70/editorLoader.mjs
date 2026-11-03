import {currentSheetLanguage,sheetLanguageKey} from './language.mjs';
// Locale can finish before the editor's larger module. Recheck it after BOTH
// loads so a late editor cannot mount in an obsolete language or session.
export async function currentSheetEditor(getLanguage,isCurrent,loadEditor,loadLanguage){
 if(!isCurrent())return null;
 const [editor,warm]=await Promise.all([loadEditor(),currentSheetLanguage(getLanguage,isCurrent,loadLanguage)]);
 if(!isCurrent())return null;
 const pack=warm?.locale===sheetLanguageKey(getLanguage())?warm:await currentSheetLanguage(getLanguage,isCurrent,loadLanguage);
 if(!pack||!isCurrent())return null;
 return {editor,pack};
}
