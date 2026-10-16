import {useEffect,useState} from 'react';
import type {LocalCapabilities} from '../local/localClient.mjs';
import type {Profile86} from './types';
import {useLocalSession} from '../local/useLocalSession';
import {contextOf,contextKey} from '../context74/model.mjs';
import {read86} from './storage';
export function useProfile86(cap:LocalCapabilities|null){const session=useLocalSession(s=>s.session),token=useLocalSession(s=>s.token),[profile,setProfile]=useState<Profile86|null>(null);let scope='';try{if(token)scope=contextKey(contextOf(session,cap));}catch{/* No trusted context. */}useEffect(()=>{let live=true;setProfile(null);if(scope)void read86(scope).then(d=>{if(live)setProfile(d?.profile||null);}).catch(()=>{});return()=>{live=false;};},[scope]);return profile;}
