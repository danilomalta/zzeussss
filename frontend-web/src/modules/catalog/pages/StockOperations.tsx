import {useEffect,useState} from 'react';
import type {LocalProduct,LocalLocation} from '../../../core/local/localClient.mjs';
import {createStockMutations,prepareStockOperation} from '../../../core/local/stockMutations.mjs';
import type {StockKind,StockReceipt,StockHistory,StockInput} from '../../../core/local/stockMutations.mjs';
import {exactQuantity as q} from '../../../core/local/operationsRead.mjs';
import {useReadTask,ReadState,ReadSection,Pages} from '../../production/pages/readPanel';
import {StockPending,useStockMutations} from './StockPending';
import {useLocalSession} from '../../../core/local/useLocalSession';
const client=createStockMutations();
const labels={entry:'Entrada de estoque',transfer:'Transferência entre locais',loss:'Perda de estoque'};
function Receipt({v}:{v:StockReceipt}){return <dl className="operation-metrics"><div><dt>Operação</dt><dd>{labels[v.input.kind]} · {v.input.operation_id}</dd></div><div><dt>Produto / quantidade</dt><dd>{v.input.product_id} · {q(v.input.quantity_milli)} {v.unit}</dd></div><div><dt>Origem / destino</dt><dd>{v.input.from_location_id||'Sem origem'} → {v.input.to_location_id||'Sem destino'}</dd></div><div><dt>Motivo</dt><dd>{v.input.reason}</dd></div><div><dt>Responsável / aparelho / data</dt><dd>{v.actor_id} · {v.device_id} · {v.occurred_at}</dd></div></dl>;}
function MovementForm({kind,products,locations}:{kind:StockKind;products:LocalProduct[];locations:LocalLocation[]}){
 const {blocked,submit}=useStockMutations();const token=useLocalSession(s=>s.token);
 const [product,setProduct]=useState(''),[from,setFrom]=useState(''),[to,setTo]=useState(''),[quantity,setQuantity]=useState(''),[reason,setReason]=useState(''),[review,setReview]=useState<StockInput|null>(null),[error,setError]=useState('');
 useEffect(()=>{setProduct('');setFrom('');setTo('');setQuantity('');setReason('');setReview(null);setError('');},[token]);
 useEffect(()=>{setReview(null);},[products,locations]);
 const selected=products.find(p=>p.id===product);
 function change(set:(v:string)=>void,v:string){set(v);setReview(null);setError('');}
 function prepare(){return prepareStockOperation(selected!,{kind,from,to,quantity,reason},'review');}
 return <ReadSection title={labels[kind]}><p>{kind==='entry'?'Acrescenta saldo físico. Recebimento de compra deve usar o fluxo próprio de compras.':kind==='transfer'?'Retira do local de origem e acrescenta no destino na mesma transação.':'Retira saldo físico por perda declarada; informe o motivo.'} O servidor confere referências, acesso e saldo livre novamente.</p>
 <form className="operation-form" onSubmit={e=>{e.preventDefault();try{setReview(prepare());setError('');}catch(e){setError(e instanceof Error?e.message:'Confira os campos.');}}}>
 <label className="brand-field">Produto desta página<select required disabled={blocked} value={product} onChange={e=>change(setProduct,e.target.value)}><option value="">Selecione</option>{products.map(p=><option key={p.id} value={p.id}>{p.name} · {p.unit}</option>)}</select></label>
 {kind!=='entry'&&<label className="brand-field">Origem<select required disabled={blocked} value={from} onChange={e=>change(setFrom,e.target.value)}><option value="">Selecione</option>{locations.map(l=><option key={l.id} value={l.id}>{l.name}</option>)}</select></label>}
 {kind!=='loss'&&<label className="brand-field">Destino<select required disabled={blocked} value={to} onChange={e=>change(setTo,e.target.value)}><option value="">Selecione</option>{locations.map(l=><option key={l.id} value={l.id}>{l.name}</option>)}</select></label>}
 <label className="brand-field">Quantidade na unidade {selected?.unit||'do produto'}<input required disabled={blocked} inputMode="decimal" value={quantity} onChange={e=>change(setQuantity,e.target.value)}/></label>
 <label className="brand-field">Motivo<input required maxLength={255} disabled={blocked} value={reason} onChange={e=>change(setReason,e.target.value)}/></label><button className="brand-secondary" disabled={blocked}>Revisar movimentação</button></form>
 {error&&<p role="alert">{error}</p>}{review&&selected&&<div><p>Confirme {labels[kind].toLowerCase()}: {q(review.quantity_milli)} {selected.unit}, produto {selected.name}, origem {from||'—'}, destino {to||'—'}. Motivo: {reason.trim()}.</p><button className="brand-primary" disabled={blocked} onClick={()=>void submit(()=>({kind,input:{...review,operation_id:crypto.randomUUID()}})).then(ok=>{if(ok){setReview(null);setQuantity('');}})}>Confirmar movimentação</button></div>}
 </ReadSection>;
}
function Reads(){
 const receipt=useReadTask<StockReceipt>(),history=useReadTask<StockHistory>();
 const [op,setOp]=useState(''),[product,setProduct]=useState(''),[location,setLocation]=useState(''),[offset,setOffset]=useState(0);
 function load(n=0){setOffset(n);void history.run(t=>client.history(t,product.trim(),location.trim(),n));}
 return <><ReadSection title="Consultar recibo original"><p>O recibo pertence ao operador e aparelho que registraram a operação. Consulta não movimenta estoque nem recalcula o resultado pelo saldo atual.</p><form className="operation-form" onSubmit={e=>{e.preventDefault();void receipt.run(t=>client.receipt(t,op.trim()));}}><label className="brand-field">ID da operação<input required maxLength={128} value={op} onChange={e=>{setOp(e.target.value);receipt.clear();}}/></label><button className="brand-secondary" disabled={receipt.busy}>Consultar recibo</button></form><ReadState task={receipt}/>{receipt.data&&<Receipt v={receipt.data}/>}</ReadSection>
 <ReadSection title="Histórico de movimentações manuais"><p>Entradas, transferências e perdas da empresa nesta loja, incluindo outros operadores. Vendas, compras, inventários e produção possuem registros próprios; esta consulta não é o extrato completo do estoque.</p><form className="operation-form" onSubmit={e=>{e.preventDefault();load();}}><label className="brand-field">ID do produto<input required maxLength={128} value={product} onChange={e=>{setProduct(e.target.value);history.clear();}}/></label><label className="brand-field">ID do local<input required maxLength={128} value={location} onChange={e=>{setLocation(e.target.value);history.clear();}}/></label><button className="brand-secondary" disabled={history.busy}>Consultar histórico</button></form><ReadState task={history}/>{history.data&&<><p>{history.data.total_count} movimentações. Transferências aparecem uma vez em cada local envolvido.</p>{history.data.items.map(v=><Receipt key={v.input.operation_id} v={v}/>)}{!history.data.items.length&&<p>Nenhuma movimentação nesta página.</p>}<Pages offset={offset} more={history.data.has_more} busy={history.busy} change={load}/></>}</ReadSection></>;
}
export default function StockOperations({products,locations}:{products:LocalProduct[];locations:LocalLocation[]}){return <StockPending>{(['entry','transfer','loss'] as const).map(kind=><MovementForm key={kind} kind={kind} products={products} locations={locations}/>)}<Reads/></StockPending>;}
