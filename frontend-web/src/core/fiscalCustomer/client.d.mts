export interface CustomerCredentialStatus {revision:number;operation_id:string;profile_id:string;profile_revision:number;environment:string;token_present:boolean;csc_present:boolean;production_blocked:true}
export interface CustomerCredentialInput {operation_id:string;expected_revision:number;profile_revision:number;token:string;csc:string}
export interface ProviderDiagnostic {provider:"external";environment:"homologation";certificate_type:"A1";token_present:boolean;csc_present:boolean}
export interface OnboardingDiagnostic {provider_configuration?:ProviderDiagnostic;profile_revision:number;configuration_valid:boolean;production_blocked:boolean;transmission_performed:boolean;official_authorization:boolean;blockers:string[]}
export interface ProfileInput {operation_id:string;profile_id:string;expected_revision:number;cnpj:string;legal_name:string;state:string;municipality_ibge:string;state_registration:string;crt:string;model:string;environment:string;series:number}
export interface Profile {input:ProfileInput;revision:number;created_at:string;readiness:{blockers:string[]}}
export interface Capabilities {transmission_available:boolean;nfse_available:boolean;sped_generation_available:boolean;production_cancellation_available:boolean;production_model?:string}
export interface Requirements {uf:string;reference_date:string;issuer_requirements:string;developer_requirements:string;limitations:string;sources:{title:string;url:string}[]}
export class FiscalCustomerError extends Error {code:string;status:number;constructor(code:string,status?:number)}
export const states:readonly string[];
export function profileMatches(profile:Profile,input:ProfileInput):boolean;
export function createFiscalCustomer(options:{token:string;isCurrent:()=>boolean;fetcher?:typeof fetch}): {providerStatus:(id:string)=>Promise<CustomerCredentialStatus>;saveProviderCredentials:(id:string,input:CustomerCredentialInput)=>Promise<CustomerCredentialStatus>;onboarding:(id:string)=>Promise<OnboardingDiagnostic>;capabilities:()=>Promise<Capabilities>;requirements:(uf:string)=>Promise<Requirements>;read:(id:string,revision?:number)=>Promise<Profile>;save:(input:ProfileInput)=>Promise<Profile>};
