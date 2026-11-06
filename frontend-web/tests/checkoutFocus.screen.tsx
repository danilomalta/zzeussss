// Test harness: actual product components, actual session/capability APIs.
// Only the upstream Focus authority is simulated by the disposable Go fixture.
import React,{useState} from 'react';
import {createRoot} from 'react-dom/client';
import i18n from 'i18next';
import {initReactI18next} from 'react-i18next';
import Checkout96 from '../src/modules/pos/pages/Checkout96';
import CheckoutFiscalStatus from '../src/modules/pos/pages/CheckoutFiscalStatus';
import {LocalAccessProvider} from '../src/core/local/LocalAccess';
import {useLocalSession} from '../src/core/local/useLocalSession';

void i18n.use(initReactI18next).init({lng:'pt-BR',fallbackLng:'pt-BR',resources:{'pt-BR':{translation:{}}},interpolation:{escapeValue:false}});
let root:ReturnType<typeof createRoot>|null=null;
function Screen({data}:{data:any}){
 const [received,onReceived]=useState(''),[customer,onCustomer]=useState({document:'',club:'skip'} as any),[receipt,setReceipt]=useState<any>(null);
 const cart=[{product:data.product,location_id:data.location,quantity_milli:1000}];
 return <LocalAccessProvider><Checkout96 scope={JSON.stringify([data.session.tenant_id,data.session.store_id,data.session.identity_id,data.session.device_id])} source={data.source} cash="turn-one" token={data.token} cart={cart} customer={customer} onCustomer={onCustomer} received={received} onReceived={onReceived} onCash={()=>{throw Error('unexpected nonfiscal fallback');}} onFinal={r=>{(window as any).__receipt=r;setReceipt(r);}} onExposure={()=>{}} disabled={false} active={true} focusDiscount={false}/>{receipt&&<CheckoutFiscalStatus checkout={receipt.sale_id}/>}</LocalAccessProvider>;
}
(window as any).start=(data:any)=>{
 root?.unmount();(window as any).__receipt=null;
 useLocalSession.setState({token:data.token,session:data.session});
 root=createRoot(document.getElementById('root')!);root.render(<Screen data={data}/>);
};
