import {useEffect,useRef,useState} from 'react';
import {parseDiagnostic,diagnosticWarnings} from '../../../core/customerOperations/diagnostic.mjs';
import type {Diagnostic} from '../../../core/customerOperations/diagnostic.mjs';
import pt from '../../../core/customerOperations/pt-BR.json';
import en from '../../../core/customerOperations/en.json';
import es from '../../../core/customerOperations/es.json';
import fr from '../../../core/customerOperations/fr.json';
import zh from '../../../core/customerOperations/zh-CN.json';
import {setupLocale} from '../../../core/companyRegistration/journey.mjs';
export function CustomerOperations({locale,sessionKey}:{locale:string;sessionKey:string}) {
 const ow:Record<string,string>=({'pt-BR':pt,en,es,fr,'zh-CN':zh} as Record<string,Record<string,string>>)[setupLocale(locale)]||pt;
 const [report,setReport]=useState<Diagnostic|null>(null),[invalid,setInvalid]=useState(false),[now,setNow]=useState(Date.now()/1000);
 const generation=useRef(0),input=useRef<HTMLInputElement>(null);
 useEffect(()=>{generation.current++;setReport(null);setInvalid(false);if(input.current)input.current.value='';return()=>{generation.current++;};},[sessionKey]);
 useEffect(()=>{const timer=setInterval(()=>setNow(Date.now()/1000),30000);return()=>clearInterval(timer);},[]);
 async function load(file:File|undefined){const g=++generation.current;setReport(null);setInvalid(false);if(!file)return;try{if(file.size>4096)throw Error();const r=parseDiagnostic(await file.text());if(g===generation.current){setReport(r);setNow(Date.now()/1000);}}catch{if(g===generation.current)setInvalid(true);}finally{if(input.current)input.current.value='';}}
 const warnings=diagnosticWarnings(report,now);
 return <section className="brand-panel" aria-labelledby="customer-operations-title"><h2 id="customer-operations-title">{ow.title}</h2><p>{ow.scope}</p><p role="note"><strong>{ow.lostKey}</strong></p><p>{ow.refresh}</p>
 <label>{ow.load}<input ref={input} type="file" accept="application/json,.json" onChange={e=>void load(e.target.files?.[0])}/></label>
 {invalid&&<p role="alert">{ow.invalid}</p>}
 <div role="status">{warnings.map(k=><p key={k}>{ow[k]}</p>)}{report&&!warnings.length&&<p>{ow.healthy}</p>}</div>
 {report&&<dl><dt>{ow.version}</dt><dd>{report.build}</dd><dt>{ow.schema}</dt><dd>{report.schema_version}</dd><dt>{ow.vault}</dt><dd>{ow[report.vault]}</dd><dt>{ow.backup}</dt><dd>{report.backup_unix?new Date(report.backup_unix*1000).toLocaleString(locale):ow.unavailable}</dd><dt>{ow.tls}</dt><dd>{ow[report.tls]||ow.tlsInvalid}</dd></dl>}
 </section>;
}
