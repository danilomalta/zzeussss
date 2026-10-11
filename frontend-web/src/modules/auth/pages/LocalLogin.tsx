import {useNavigate} from 'react-router-dom';
import SignInCard from '../../../shared/brand/SignInCard';
import {useLocalSession} from '../../../core/local/useLocalSession';
export default function LocalLogin() {
  const login=useLocalSession(state=>state.login),notice=useLocalSession(state=>state.notice),navigate=useNavigate();
  return <SignInCard mode="local" notice={!!notice} onSubmit={async(id,password)=>{await login(id,password);navigate('/local/home',{replace:true});}}/>;
}
