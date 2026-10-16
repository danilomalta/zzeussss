import {useEffect,useState} from 'react';
import type {LocalCapabilities} from '../local/localClient.mjs';
import type {Profile86} from './types';
import {useLocalSession} from '../local/useLocalSession';
import {contextOf,contextKey} from '../context74/model.mjs';
import {readProfile87} from './profile87';
export function useProfile86(cap:LocalCapabilities|null){const session=useLocalSession(s=>s.session),token=useLocalSession(s=>s.token),[state,setState]=useState<{scope:string;profile:Profile86|null}>({scope:'',profile:null});let scope='';try{if(token)scope=contextKey(contextOf(session,cap));}catch{/* No trusted context. */}useEffect(()=>{let live=true;const refresh=()=>{if(scope)void readProfile87(scope).then(d=>{if(live)setState({scope,profile:d?.profile||null});}).catch(()=>{if(live)setState({scope,profile:null});});};refresh();window.addEventListener('titan-profile87',refresh);return()=>{live=false;window.removeEventListener('titan-profile87',refresh);};},[scope]);return state.scope===scope?state.profile:null;}
