import {useEffect,useId,useRef,useState} from 'react';
import type {RefObject} from 'react';
import {useTranslation} from 'react-i18next';
import {recognize96,frame96,image96} from '../../../core/local/ocr96';
import {words106} from '../../../core/local/words106';
import {emptyPaper106,readPaper106,checkedPaper106,paperText106} from '../../../core/local/paper106.mjs';
import type {PaperFields106} from '../../../core/local/paper106.mjs';
import './paper106.css';

export default function OCR96({video,onRead,disabled=false}:{video?:RefObject<HTMLVideoElement>;onRead:(text:string)=>void;disabled?:boolean}){
 const {i18n}=useTranslation(),w=words106(i18n.resolvedLanguage||i18n.language),id=useId();
 const [busy,setBusy]=useState(false),[progress,setProgress]=useState(0),[error,setError]=useState(''),[confidence,setConfidence]=useState<number|null>(null);
 const [raw,setRaw]=useState(''),[fields,setFields]=useState<PaperFields106|null>(null),[reviewed,setReviewed]=useState(false),[used,setUsed]=useState(false);
 const abort=useRef<AbortController|null>(null),alive=useRef(true),generation=useRef(0),latch=useRef(false),heading=useRef<HTMLHeadingElement>(null),applyLatch=useRef(false);
 useEffect(()=>{alive.current=true;return()=>{alive.current=false;generation.current++;abort.current?.abort();};},[]);
 useEffect(()=>{if(disabled){generation.current++;abort.current?.abort();setFields(null);setRaw('');setReviewed(false);setConfidence(null);setUsed(false);setError('');}},[disabled]);
 useEffect(()=>{if(fields)heading.current?.focus();},[fields!==null]);
 const dirty=!!(raw||fields);
 useEffect(()=>{if(!dirty)return;const f=(e:BeforeUnloadEvent)=>{e.preventDefault();e.returnValue='';};window.addEventListener('beforeunload',f);return()=>window.removeEventListener('beforeunload',f);},[dirty]);
 function replaceOK(){return !dirty||window.confirm(w.discardPaper106);}
 function present(text:string){setFields(readPaper106(text));setRaw('');setReviewed(false);setError('');setUsed(false);applyLatch.current=false;}
 function edit(key:keyof PaperFields106,value:string){setFields(p=>({...p||emptyPaper106(),[key]:value}));setReviewed(false);setError('');applyLatch.current=false;}
 let valid=false;try{if(fields){checkedPaper106(fields);valid=true;}}catch{/* Missing values require review. */}
 async function read(input:Blob|HTMLCanvasElement){
  if(latch.current||disabled){if(input instanceof HTMLCanvasElement){input.width=0;input.height=0;}return;}
  latch.current=true;setBusy(true);setError('');setProgress(0);setConfidence(null);setFields(null);setRaw('');setReviewed(false);setUsed(false);
  const c=new AbortController(),g=++generation.current;abort.current=c;let canvas:HTMLCanvasElement|undefined;
  try{canvas=input instanceof Blob?await image96(input):input;if(c.signal.aborted)throw Error('ocrCancelled96');const result=await recognize96(canvas,c.signal,n=>{if(alive.current&&g===generation.current)setProgress(n);});
   if(alive.current&&g===generation.current&&!c.signal.aborted){present(result.text);setConfidence(Number.isFinite(result.confidence)?result.confidence:null);}
  }catch(e){if(alive.current&&g===generation.current)setError(e instanceof Error?e.message:'ocrUnavailable96');}
  finally{if(canvas){canvas.width=0;canvas.height=0;}latch.current=false;if(alive.current)setBusy(false);}
 }
 function apply(){if(applyLatch.current||disabled||busy||!fields||!reviewed)return;applyLatch.current=true;try{const text=paperText106(fields);onRead(text);setFields(null);setRaw('');setConfidence(null);setReviewed(false);setUsed(true);setError('');}catch{applyLatch.current=false;setError('invalidPaper106');}}
 return <section className="ocr96 paper106" aria-label={w.title106}>
  <h3>{w.title106}</h3><p>{w.help106}</p><p>{w.ocrLocal96}</p>
  <div className="paper106-actions">
   {video&&<button type="button" disabled={disabled||busy} onClick={()=>{if(!replaceOK())return;try{if(!video.current)throw Error();void read(frame96(video.current));}catch{setError('cameraUnavailable');}}}>{w.ocrFrame96}</button>}
   <label className="paper106-file">{w.ocrImage96}<input type="file" name="ocrImage96" accept="image/png,image/jpeg,image/webp" disabled={disabled||busy} onChange={e=>{const f=e.target.files?.[0];e.target.value='';if(!f||!replaceOK())return;if(f.size>10*1024*1024||!['image/png','image/jpeg','image/webp'].includes(f.type)){setError('invalidPaper106');return;}void read(f);}}/></label>
   {busy&&<button type="button" onClick={()=>abort.current?.abort()}>{w.cancel}</button>}
  </div>
  <details><summary>{w.paste106}</summary><label htmlFor={id+'raw'}>{w.raw106}</label><textarea id={id+'raw'} maxLength={12000} value={raw} disabled={disabled||busy} onChange={e=>{setRaw(e.target.value);setFields(null);setReviewed(false);setConfidence(null);setUsed(false);}}/>
   <button type="button" disabled={disabled||busy||!raw.trim()} onClick={()=>{if(fields&&!window.confirm(w.discardPaper106))return;try{present(raw);setConfidence(null);}catch{setError('invalidPaper106');}}}>{w.read106}</button>
  </details>
  {busy&&<p role="status">{w.ocrReading96} {progress}%</p>}
  {confidence!==null&&<p role="status">{w.confidence106}: {new Intl.NumberFormat(i18n.language,{maximumFractionDigits:0}).format(confidence)}% {confidence<80&&w.low106}</p>}
  {error&&<p role="alert">{w[error]||w.ocrUnavailable96}</p>}{used&&<p role="status">{w.used106}</p>}
  {fields&&<div className="paper106-review"><h4 tabIndex={-1} ref={heading}>{w.review106}</h4><p>{w.missing106}</p><div className="paper106-grid">
   <label htmlFor={id+'amount'}>{w.amount106}<input id={id+'amount'} inputMode="decimal" maxLength={32} value={fields.amount} disabled={disabled||busy} onChange={e=>edit('amount',e.target.value)}/></label>
   <div style={{display:'grid',gap:8}}><label htmlFor={id+'method'}>{w.method106}</label><select id={id+'method'} value={fields.method} disabled={disabled||busy} onChange={e=>edit('method',e.target.value)}><option value="">{w.choose106}</option>{['debit','credit','pix'].map(m=><option key={m} value={m}>{w[m+'106']}</option>)}</select></div>
   <label htmlFor={id+'reference'}>{w.reference106}<input id={id+'reference'} autoComplete="off" maxLength={40} value={fields.reference} disabled={disabled||busy} onChange={e=>edit('reference',e.target.value)}/></label>
   <label htmlFor={id+'at'}>{w.at106}<input id={id+'at'} type="datetime-local" min="2000-01-01T00:00" max="2199-12-31T23:59" value={fields.at} disabled={disabled||busy} onChange={e=>edit('at',e.target.value)}/></label>
  </div><label className="paper106-check"><input type="checkbox" checked={reviewed} disabled={disabled||busy||!valid} onChange={e=>setReviewed(e.target.checked)}/>{w.checked106}</label>
  <div className="paper106-actions"><button type="button" className="brand-primary" disabled={disabled||busy||!valid||!reviewed} onClick={apply}>{w.use106}</button><button type="button" disabled={disabled||busy} onClick={()=>{if(!replaceOK())return;setFields(null);setRaw('');setReviewed(false);setConfidence(null);setError('');}}>{w.clear106}</button></div></div>}
 </section>;
}
