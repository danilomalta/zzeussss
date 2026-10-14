import {useState} from 'react';
import {useNavigate} from 'react-router-dom';
import {useTranslation} from 'react-i18next';
import {useLocalSession} from '../../core/local/useLocalSession';
export default function LogoutButton82(){
 const {t}=useTranslation(),navigate=useNavigate(),endSession=useLocalSession(state=>state.endSession);
 const [busy,setBusy]=useState(false);
 async function logout(){
  if(busy)return;
  setBusy(true);
  try{await endSession();}finally{navigate('/local/login',{replace:true});}
 }
 return <button type="button" className="brand-secondary" disabled={busy} onClick={()=>void logout()} style={{display:'inline-flex',alignItems:'center',gap:8,whiteSpace:'nowrap'}}>
  {busy?<span className="brand-spinner" aria-hidden="true"/>:<svg aria-hidden="true" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><path d="M9 4H4v16h5M14 7l5 5-5 5M8 12h11"/></svg>}
  {t('workspace.logout')}
 </button>;
}
