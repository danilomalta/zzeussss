export interface RegistrationInput {operation_id:string;expected_revision:number;stage:'pre_registration'|'company_profile';company_name:string;responsible:string;cnpj:string;legal_name:string;uf:string;state_registration:string;municipality_ibge:string;crt:string;provider:string;environment:string}
export interface RegistrationRecord {input:RegistrationInput;revision:number;created_at:string;repeated:boolean}
export class RegistrationError extends Error {code:string;status:number}
export function automaticCompanySource():{enabled:false;available:false;thirdPartyDisclosure:false};
export function createRegistrationClient(options:{token:string;isCurrent:()=>boolean;fetcher?:typeof fetch}):{read:(operation?:string)=>Promise<RegistrationRecord>;save:(input:RegistrationInput)=>Promise<RegistrationRecord>};

export function prefillFiscalProfile<T extends object>(profile:T,company:RegistrationInput):T;
