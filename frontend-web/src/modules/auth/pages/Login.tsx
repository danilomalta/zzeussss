import {useNavigate} from 'react-router-dom';
import SignInCard from '../../../shared/brand/SignInCard';
import {useAuthStore} from '../../../core/auth/useAuthStore';
export default function Login() {
  const login=useAuthStore(state=>state.login),navigate=useNavigate();
  return <SignInCard mode="online" onSubmit={async(email,password)=>{await login(email,password);navigate('/',{replace:true});}}/>;
}
