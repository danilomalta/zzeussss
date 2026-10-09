import {useState} from 'react';
import {Link} from 'react-router-dom';
import {useLocalAccess} from '../../../core/local/LocalAccess';
import {createStockRead} from '../../../core/local/stockRead.mjs';
import type {StockBalance,Reservations} from '../../../core/local/stockRead.mjs';
import {exactQuantity as q} from '../../../core/local/operationsRead.mjs';
import {useReadTask,ReadState,ReadSection,Pages} from '../../production/pages/readPanel';
const client=createStockRead();
function Metrics({balance}:{balance:StockBalance}){return <dl className="operation-metrics"><div><dt>Saldo físico</dt><dd>{q(balance.physical_milli)} {balance.unit}</dd></div><div><dt>Reservado global</dt><dd>{q(balance.reserved_milli)} {balance.unit}</dd></div><div><dt>Livre</dt><dd>{q(balance.free_milli)} {balance.unit}</dd></div></dl>;}
export default function LocalStockAvailability(){
 const {capabilities}=useLocalAccess();
 const canReservations=!!capabilities?.permissions.includes('manage_production');
 const canCatalog=!!capabilities?.permissions.includes('view_catalog');
 const balance=useReadTask<StockBalance>(),reservations=useReadTask<Reservations>();
 const [product,setProduct]=useState(''),[location,setLocation]=useState(''),[offset,setOffset]=useState(0);
 function clear(){balance.clear();reservations.clear();setOffset(0);}
 function loadReservations(value=0){setOffset(value);balance.clear();void reservations.run(t=>client.reservations(t,product.trim(),location.trim(),value));}
 return <main className="brand-app-content operation-page"><header><p className="brand-eyebrow">ESTOQUE LOCAL</p><h1>Saldo disponível e reservas</h1><p>Saldo físico = reservado + livre. Uma consulta é uma fotografia do estoque; operações posteriores conferem o saldo novamente.</p>{canCatalog&&<Link to="/local/catalog" className="brand-secondary">Abrir catálogo, locais e entradas</Link>}</header>
 <ReadSection title="Produto e local"><form className="operation-form" onSubmit={e=>{e.preventDefault();reservations.clear();void balance.run(t=>client.balance(t,product.trim(),location.trim()));}}><label className="brand-field">ID do produto<input required maxLength={128} value={product} onChange={e=>{setProduct(e.target.value);clear();}}/></label><label className="brand-field">ID do local<input required maxLength={128} value={location} onChange={e=>{setLocation(e.target.value);clear();}}/></label><button className="brand-primary" disabled={balance.busy||reservations.busy}>Consultar saldo</button>{canReservations&&<button type="button" className="brand-secondary" disabled={!product.trim()||!location.trim()||balance.busy||reservations.busy} onClick={()=>loadReservations()}>Detalhar reservas</button>}</form><ReadState task={balance}/><ReadState task={reservations}/>
 {balance.data&&<Metrics balance={balance.data}/>}{reservations.data&&<Metrics balance={reservations.data.balance}/>}
 {!canReservations&&<p>Detalhes das reservas exigem também permissão de produção. A API confere os acessos.</p>}
 </ReadSection>
 {reservations.data&&<ReadSection title="Origem das reservas ativas"><p>{reservations.data.total_count} reservas ativas no produto e local. O saldo acima é global; a tabela mostra somente esta página.</p><div className="operation-table-wrap"><table className="operation-table"><caption>Reservas de ordens de produção</caption><thead><tr><th>Ordem e versão</th><th>Responsável</th><th>Quantidade</th><th>Criada em</th></tr></thead><tbody>{reservations.data.items.map(r=><tr key={r.reservation_id}><td><span className="operation-id">{r.order_id}</span><small>Versão: {r.version_id}<br/>Reserva: {r.reservation_id}</small></td><td className="operation-id">{r.responsible_id}</td><td>{q(r.quantity_milli)} {r.unit}</td><td>{r.created_at}</td></tr>)}</tbody></table></div>{reservations.data.items.length===0&&<p>Nenhuma reserva ativa nesta página.</p>}<Pages offset={offset} more={reservations.data.has_more} busy={reservations.busy} change={loadReservations}/></ReadSection>}
 </main>;
}
