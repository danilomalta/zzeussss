import {useEffect,useState} from 'react';
import {useSectors86,useForm86} from '../../../../core/sectors86/context';
import {can86} from '../../../../core/sectors86/rules.mjs';
import {proposal87,readProfile87,recommended87,validProfile87,writeProfile87} from '../../../../core/sectors86/profile87';
import BusinessProfileFields87 from '../../../../shared/brand/BusinessProfileFields87';
import {Button86,Panel86} from './UI';
export default function Profile86(){
 const {data,cap,t,manager,ask,commit,notice}=useSectors86(),f=useForm86('profile87');
 const [revision,setRevision]=useState(0),[ready,setReady]=useState(false),[busy,setBusy]=useState(false),[failure,setFailure]=useState('');
 useEffect(()=>{let live=true;setReady(false);void readProfile87(data.scope).then(r=>{if(live){setRevision(r?.revision||0);setReady(true);}}).catch(()=>{if(live)setFailure('saveFailed');});return()=>{live=false;};},[data.scope]);
 let profile=data.profile.configured?data.profile:recommended87('other');try{const p=JSON.parse(f.get('value','null'));if(validProfile87(p))profile=p;}catch{/* Invalid draft is never applied. */}
 const proposal=proposal87();
 async function persist(){if(!manager||!can86(cap,'sectors'))return;setBusy(true);setFailure('');try{await writeProfile87(data.scope,profile,revision);setRevision(v=>v+1);commit('saveProfile',profile.activity,d=>{d.profile=profile;delete d.forms.profile87;},true);notice('profileSaved87');}catch(e){setFailure(e instanceof Error?e.message:'saveFailed');}finally{setBusy(false);}}
 return <Panel86 title={t('profileTitle87')} help={t('profileHelp87')} wide>{!manager?<p>{t('askManagerSetup')}</p>:<><p>{t('profileLocal87')}</p>{proposal&&<Button86 onClick={()=>f.set('value',JSON.stringify(proposal))}>{t('useProposal87')}</Button86>}<BusinessProfileFields87 value={profile} onChange={p=>f.set('value',JSON.stringify(p))} t={t}/>{failure&&<p role="alert">{t(failure)}</p>}<div className="wf86-actions"><Button86 primary disabled={!ready||busy||data.demo} onClick={()=>ask(t('saveProfile'),()=>void persist())}>{t(busy?'saving':'saveProfile')}</Button86></div></>}</Panel86>;
}
