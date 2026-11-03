import type {RegistrationRecord} from './client.mjs';
export function companySetupStatus(record:RegistrationRecord|null):Readonly<{preSaved:boolean;fullSaved:boolean}>;
export function setupLocale(locale:string):'pt-BR'|'en'|'es'|'fr'|'zh-CN';
