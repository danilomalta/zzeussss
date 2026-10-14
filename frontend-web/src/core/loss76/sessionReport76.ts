import {useLocalSession} from '../local/useLocalSession';
import {clearReport76} from './report76.mjs';
// One ephemeral handoff; never store tokens, reports or costs in browser storage.
useLocalSession.subscribe((state,previous)=>{if(state.token!==previous.token)clearReport76();});
