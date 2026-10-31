const fields=['operation_id','expected_revision','stage','company_name','responsible','cnpj','legal_name','uf','state_registration','municipality_ibge','crt','provider','environment'];
export class RegistrationError extends Error {constructor(code,status=0){super(code);this.code=code;this.status=status;}}
export function automaticCompanySource(){return Object.freeze({enabled:false,available:false,thirdPartyDisclosure:false});}
export function createRegistrationClient({token,isCurrent,fetcher=fetch}){
 const current=()=>{if(!token||!isCurrent())throw new RegistrationError('session',401);};
 async function request(input,operation){current();const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),15000);
  try{const response=await fetcher('/local/v1/company/registration'+(operation?'?operation_id='+encodeURIComponent(operation):''),{method:input?'POST':'GET',headers:{Authorization:'Bearer '+token,...(input?{'Content-Type':'application/json'}:{})},credentials:'omit',cache:'no-store',signal:controller.signal,...(input?{body:JSON.stringify(input)}:{})});current();
   if(!response.ok)throw new RegistrationError(response.status===401?'session':response.status===403?'denied':response.status===409?'conflict':'error',response.status);
   const text=await response.text();current();if(text.length>32768)throw new RegistrationError('error');const value=JSON.parse(text);
   if(!value?.input||Object.keys(value.input).sort().join()!==[...fields].sort().join()||Object.keys(value).sort().join()!==['input','revision','created_at','repeated'].sort().join()||typeof value.repeated!=='boolean'||typeof value.created_at!=='string'||!Number.isSafeInteger(value.revision)||value.revision<0||!fields.every(k=>k==='expected_revision'?Number.isSafeInteger(value.input[k]):typeof value.input[k]==='string'))throw new RegistrationError('error');
   if(operation&&value.input.operation_id!==operation)throw new RegistrationError('conflict');return value;
  }catch(e){if(e instanceof RegistrationError)throw e;throw new RegistrationError('uncertain');}finally{clearTimeout(timer);}
 }
 return {read:operation=>request(null,operation),async save(input){if(Object.keys(input).sort().join()!==[...fields].sort().join())throw new RegistrationError('error',400);const value=await request(input);if(value.input.operation_id!==input.operation_id||value.revision!==input.expected_revision+1||!fields.every(k=>value.input[k]===input[k]))throw new RegistrationError('conflict');return value;}};
}

export function prefillFiscalProfile(profile,company){
 const mapping={cnpj:'cnpj',legal_name:'legal_name',state:'uf',municipality_ibge:'municipality_ibge',state_registration:'state_registration',crt:'crt'};
 const next={...profile};for(const [key,source]of Object.entries(mapping))if(!next[key]&&typeof company?.[source]==='string')next[key]=company[source];return next;
}
