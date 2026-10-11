import { createContext, useCallback, useContext, useEffect, useState } from 'react';
import type {ReactNode} from 'react';
import {localClient, localErrorMessage, useLocalSession} from './useLocalSession';
import type {LocalCapabilities} from './localClient.mjs';
import {LocalAPIError} from './localClient.mjs';
import {useTranslation} from 'react-i18next';
const Context=createContext<{capabilities:LocalCapabilities|null; refresh:()=>void}>({capabilities:null,refresh:()=>{}});
export const useLocalAccess=()=>useContext(Context);
export function LocalAccessProvider({children}:{children:ReactNode}) {
 const {t}=useTranslation();
 const token=useLocalSession(s=>s.token), session=useLocalSession(s=>s.session), invalidate=useLocalSession(s=>s.invalidate);
 const [capabilities,setCapabilities]=useState<LocalCapabilities|null>(null),[error,setError]=useState(''),[revision,setRevision]=useState(0),[loading,setLoading]=useState(true);
 const refresh=useCallback(()=>setRevision(v=>v+1),[]);
 useEffect(()=>{if(!token || !session)return;let live=true;setLoading(true);setCapabilities(null);
  localClient.capabilities(token,session).then(v=>{if(live){setCapabilities(v);setError('');}}).catch(e=>{if(live){setError(localErrorMessage(e));if(e instanceof LocalAPIError && e.status===401)invalidate();}}).finally(()=>{if(live)setLoading(false);});return()=>{live=false;};
 },[token,session,revision,invalidate]);

 if(!capabilities)return <main className="brand-page"><section className="brand-panel" style={{margin:40}}><h1>{t(loading?'access.checking':'access.failed')}</h1>{error && <p role="alert">{t('errors.connection')}</p>}<button className="brand-secondary" disabled={loading} onClick={refresh}>{t('access.retry')}</button></section></main>;
 return <Context.Provider value={{capabilities,refresh}}>{children}</Context.Provider>;
}
