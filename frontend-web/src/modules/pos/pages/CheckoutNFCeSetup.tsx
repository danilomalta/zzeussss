import type {CartItem} from '../../../core/local/posModel.mjs';
import type {NFCeSetup} from '../../../core/local/checkoutNFCe.mjs';

export default function CheckoutNFCeSetup({value,onChange,cart,disabled,w}:{value:NFCeSetup;onChange:(v:NFCeSetup)=>void;cart:CartItem[];disabled:boolean;w:Record<string,string>}){
 return <fieldset disabled={disabled}><legend>{w.checkoutNFCeSetup}</legend>
  <strong role="note">{w.checkoutNFCeBanner}</strong><p>{w.checkoutNFCeSetupHelp}</p>
  <label>{w.checkoutNFCeNature}<input maxLength={60} value={value.nature} onChange={e=>onChange({...value,nature:e.target.value})}/></label>
  <label>{w.checkoutNFCePresence}<select value={value.presence} onChange={e=>onChange({...value,presence:e.target.value})}><option value="">—</option><option value="1">{w.checkoutNFCeInPerson}</option><option value="4">{w.checkoutNFCeDelivery}</option></select></label>
  {cart.map((item,index)=><label key={index}>{item.product.name} — {w.checkoutNFCePolicy}<input maxLength={128} value={value.policies[index]||''} onChange={e=>onChange({...value,policies:{...value.policies,[index]:e.target.value}})}/></label>)}
  <details><summary>{w.checkoutNFCeIssuerAddress}</summary>
  {(['Street','Number','District','City','PostalCode'] as const).map(k=><label key={k}>{w['checkoutNFCe'+k]}<input autoComplete="off" maxLength={100} value={value.issuerAddress[k]||''} onChange={e=>onChange({...value,issuerAddress:{...value.issuerAddress,[k]:e.target.value}})}/></label>)}
 </details></fieldset>;
}
