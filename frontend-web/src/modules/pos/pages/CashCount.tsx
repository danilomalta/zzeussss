import { useState } from 'react';
import { cashCount, cashDenominations } from '../../../core/local/cashCount.mjs';
import { formatLocalCents } from '../../../core/local/localClient.mjs';
export default function CashCount({disabled,onTotal}:{disabled:boolean;onTotal:(value:string)=>void}) {
 const [counts,setCounts]=useState<string[]>(cashDenominations.map(()=>''));let total=0,error='';
 try{total=cashCount(counts);}catch(e){error=e instanceof Error?e.message:'Contagem inválida';}
 return <div className="pos-count"><h3>Assistente de contagem</h3><p>Informe quantas notas e moedas você contou. O saldo esperado continua oculto.</p><div className="pos-count-grid">{cashDenominations.map((value,i)=><label key={value}>{formatLocalCents(value)}<input aria-label={`Quantidade de ${formatLocalCents(value)}`} inputMode="numeric" pattern="[0-9]*" disabled={disabled} placeholder="0" value={counts[i]} onChange={e=>setCounts(v=>v.map((n,j)=>j===i?e.target.value:n))}/></label>)}</div>{error?<p role="alert">{error}</p>:<p>Total contado: <strong>{formatLocalCents(total)}</strong></p>}<button type="button" className="brand-secondary" disabled={disabled || !!error} onClick={()=>onTotal(`${Math.trunc(total/100)},${String(total%100).padStart(2,'0')}`)}>Usar esta contagem no fechamento</button></div>;
}
