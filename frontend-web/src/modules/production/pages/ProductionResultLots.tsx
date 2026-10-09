import {useState} from 'react';
import {createProductionLots} from '../../../core/local/productionLots.mjs';
import type {LotSummary} from '../../../core/local/productionLots.mjs';
import {exactQuantity as q} from '../../../core/local/operationsRead.mjs';
import {useReadTask,ReadState,ReadSection,Pages} from './readPanel';
const client=createProductionLots();
export default function ProductionResultLots(){
 const loaded=useReadTask<LotSummary>();const [resultID,setResultID]=useState(''),[offset,setOffset]=useState(0);
 function load(n=0){setOffset(n);void loaded.run(t=>client.summary(t,resultID.trim(),n));}
 return <ReadSection title="Lotes do resultado de produção"><p>Consulte a quantidade produzida e sua distribuição em lotes. Esta distribuição é um registro de rastreabilidade; não representa saldo atual disponível para venda.</p>
 <form className="operation-form" onSubmit={e=>{e.preventDefault();load();}}><label className="brand-field">ID do resultado<input required maxLength={128} value={resultID} onChange={e=>{setResultID(e.target.value);setOffset(0);loaded.clear();}}/></label><button className="brand-secondary" disabled={loaded.busy}>Consultar lotes</button></form><ReadState task={loaded}/>
 {loaded.data&&<><p className="operation-id">Resultado: {loaded.data.result_id}<br/>Produto: {loaded.data.product_id}</p><dl className="operation-metrics"><div><dt>Produzido</dt><dd>{q(loaded.data.produced_milli)} {loaded.data.unit}</dd></div><div><dt>Atribuído a lotes ativos</dt><dd>{q(loaded.data.assigned_milli)} {loaded.data.unit}</dd></div><div><dt>Ainda sem lote</dt><dd>{q(loaded.data.unassigned_milli)} {loaded.data.unit}</dd></div></dl><p>Totais globais do resultado. Lotes anulados permanecem no histórico e não entram no total atribuído.</p>
 {loaded.data.items.length===0&&<p>Nenhum lote nesta página.</p>}{loaded.data.items.map(l=><details key={l.id}><summary>{l.code} · {q(l.quantity_milli)} {l.unit} · {l.status==='recorded'?'Declarado':'Anulado'}</summary><p className="operation-id">Lote: {l.id}<br/>Responsável pelo registro: {l.created_by}<br/>Revisão: {l.revision}</p><p>Fabricação declarada: {l.manufactured_on}<br/>Validade declarada: {l.expires_on||'Não informada'}</p><p>Motivo original: {l.reason}</p><p>Validade não informada não significa validade ilimitada. Uma avaliação de qualidade deve ser consultada separadamente.</p></details>)}<Pages offset={offset} more={loaded.data.items.length===50} busy={loaded.busy} change={load}/></>}
 </ReadSection>;
}
