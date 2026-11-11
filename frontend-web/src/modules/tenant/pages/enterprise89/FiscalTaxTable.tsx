import {useEffect,useRef,useState} from 'react';
import {createTaxTableClient,TaxTableError} from '../../../../core/fiscalCustomer/taxTable.mjs';
import type {PreparedTaxTable,TaxTableResult} from '../../../../core/fiscalCustomer/taxTable.mjs';
import type {Profile} from '../../../../core/fiscalCustomer/client.mjs';
export function FiscalTaxTable({token,isCurrent,profile,words:fw}:{token:string;isCurrent:()=>boolean;profile:Profile;words:Record<string,string>}){
 const api=useRef<ReturnType<typeof createTaxTableClient>|null>(null);
 const [view,setView]=useState<PreparedTaxTable|null>(null),[result,setResult]=useState<TaxTableResult|null>(null),[busy,setBusy]=useState(false),[confirmed,setConfirmed]=useState(false),[attempted,setAttempted]=useState(false),[uncertain,setUncertain]=useState(false),[error,setError]=useState('');
 const generation=useRef(0),inFlight=useRef(false);
 useEffect(()=>{const client=createTaxTableClient({token,isCurrent,profile:{...profile.input,revision:profile.revision}});api.current=client;generation.current++;inFlight.current=false;setView(null);setResult(null);setBusy(false);setConfirmed(false);setAttempted(false);setUncertain(false);setError('');return()=>{generation.current++;client.dispose();if(api.current===client)api.current=null;};},[token,isCurrent,profile]);
 function failure(e:unknown){setError(e instanceof TaxTableError?e.code:'file');}
 async function select(file:File){
  const client=api.current;if(!client||inFlight.current||!isCurrent()||uncertain)return;const n=++generation.current;inFlight.current=true;setBusy(true);setView(null);setError('');setResult(null);setConfirmed(false);setAttempted(false);
  try{if(file.size>1024*1024)throw new TaxTableError('file');const bytes=new Uint8Array(await file.arrayBuffer());const prepared=await client.prepare(bytes);if(n===generation.current&&isCurrent())setView(prepared);}
  catch(e){if(n===generation.current&&isCurrent())failure(e);}finally{if(n===generation.current){inFlight.current=false;setBusy(false);}}
 }
 async function run(query:boolean){
  const client=api.current;if(!client||!view||!isCurrent()||inFlight.current||(!query&&(!confirmed||attempted)))return;
  const n=++generation.current;inFlight.current=true;setBusy(true);setError('');if(!query)setAttempted(true);
  try{const out=query?await client.reconcile(view):await client.importTable(view);if(n===generation.current&&isCurrent()){setResult(out);if(out.state==='confirmed'){setAttempted(true);setUncertain(false);}else if(attempted)setUncertain(true);}}
  catch(e){if(n===generation.current&&isCurrent()){failure(e);if(!query){const ambiguous=!(e instanceof TaxTableError)||e.uncertain||e.code==='session';setUncertain(ambiguous);if(!ambiguous)setAttempted(false);}}}
  finally{if(n===generation.current){inFlight.current=false;setBusy(false);}}
 }
 return <section aria-label={fw.taxTableTitle}><h3>{fw.taxTableTitle}</h3><p>{fw.taxTableHelp}</p><p role="note"><strong>{fw.taxAccountantUnapproved}</strong></p>
 <label>{fw.taxTableFile}<input type="file" accept=".json,application/json" disabled={busy||uncertain||!isCurrent()} onChange={e=>{const file=e.target.files?.[0];e.target.value='';if(file)void select(file);}}/></label>
 {error&&<p role="alert">{fw['taxTableError_'+error]||fw.taxTableError_refused}</p>}
 {uncertain&&<p role="alert">{fw.taxTableUncertain}</p>}
 {view&&<><p>{fw.revision}: {view.revision} · {view.regime}</p><p style={{overflowWrap:'anywhere'}}>SHA-256: {view.hash}</p><div className="erp89-table"><table><caption>{fw.taxTablePreview}</caption><thead><tr>{[fw.taxTableRule,fw.nfeProduct,'NCM','CFOP',fw.taxTableValidity,fw.taxTableSources,fw.taxTableActive].map((label,i)=><th key={i}>{label}</th>)}</tr></thead><tbody>{view.rows.map(row=><tr key={row.id}><td>{row.id}</td><td>{row.product}</td><td>{row.ncm}</td><td>{row.cfop}</td><td>{row.from} → {row.until||'—'}</td><td>{row.sources.map((source,i)=><p key={i}>{source}</p>)}</td><td>{fw[row.active?'yes':'no']}</td></tr>)}</tbody></table></div>
 <label><input type="checkbox" checked={confirmed} disabled={busy||attempted} onChange={e=>setConfirmed(e.target.checked)}/>{fw.taxTableConfirm}</label>
 <button type="button" disabled={busy||!confirmed||attempted||!isCurrent()} onClick={()=>void run(false)}>{fw.taxTableImport}</button>
 <button type="button" disabled={busy||!isCurrent()} onClick={()=>void run(true)}>{fw.taxTableQuery}</button>
 {result&&<p role="status">{fw[result.state==='confirmed'?'taxTableConfirmed':'taxTableUnresolved']} · {result.count}/{view.rows.length}</p>}
 </>}
 </section>;
}
