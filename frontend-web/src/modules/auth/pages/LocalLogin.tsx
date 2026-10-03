import { Link, useNavigate } from 'react-router-dom';
import SignInCard from '../../../shared/brand/SignInCard';
import { localErrorMessage, useLocalSession } from '../../../core/local/useLocalSession';

export default function LocalLogin() {
  const login = useLocalSession((state) => state.login);
  const notice = useLocalSession((state) => state.notice);
  const navigate = useNavigate();
  return <SignInCard
    credentialLabel="ID do operador"
    credentialPlaceholder="ID recebido na instalação local"
    credentialType="text"
    subtitle="Tudo para sua empresa, em um só lugar."
    notice={notice}
    signupHelp="Esta instalação já possui uma empresa. Novos operadores são vinculados pelo administrador; o cadastro comercial será oferecido no site."
    recoveryHelp="A recuperação da senha local exige verificação do dono ou administrador desta instalação. O fluxo automático ainda não está disponível."
    errorMessage={localErrorMessage}
    onSubmit={async (id, password) => { await login(id, password); navigate('/local/home', { replace: true }); }}
    secondary={<>Acesso à API online? <Link to="/login">Entrar aqui</Link></>}
  />;
}
