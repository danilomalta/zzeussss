export interface Settings80{version:1;companyName:string;logoLight:string;logoDark:string;background:string;backgroundFit:'cover'|'contain';backgroundBlur:0|3;mode:'theme'|'photo';accent:string;lowEffects:boolean;compact:boolean;fiscal:{cnpj:string;ie:string;uf:string;regime:string;series:string;nextNumber:string};equipment:{printer:string;paper:string;copies:string;connection:string};}
export function defaults80():Settings80;
export function validate80(value:unknown):Settings80;
export function image80(value:unknown,limit:number):boolean;
export function settingsKey80(context:unknown):string;
export function contrast80(a:string,b:string):number;
export function palette80(accent:string,dark:boolean):{accent:string;buttonText:string}|null;
