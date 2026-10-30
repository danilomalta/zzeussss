import {createCheckoutFiscalClient,type CheckoutFiscalRequest} from './checkoutFiscal.mjs';
import {read96,write96} from './store96';
import {useLocalSession} from './useLocalSession';

export const checkoutFiscalClient=createCheckoutFiscalClient({getCurrent:useLocalSession.getState,cache:{
 read:key=>read96<CheckoutFiscalRequest>('plans',key),
 async write(key,body){
  try{await write96('plans',key,body,JSON.stringify(null));}
  catch(error){const existing=await read96('plans',key);if(JSON.stringify(existing)!==JSON.stringify(body))throw error;}
 }
}});
