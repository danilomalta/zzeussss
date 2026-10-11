import { Link, useNavigate } from 'react-router-dom';
import SignInCard from '../../../shared/brand/SignInCard';
import { localErrorMessage, useLocalSession } from '../../../core/local/useLocalSession';

export default function LocalLogin() {
  const login = useLocalSession((state) => state.login);
  const notice = useLocalSession((state) => state.notice);
  const navigate = useNavigate();
  return <SignInCard
    credentialLabel="Usuário desta instalação"
    credentialPlaceholder="Código de acesso fornecido pela empresa"
    credentialHelp="Use o código do seu usuário local. O acesso por e-mail fica na área online."
    credentialType="text"
    subtitle="Tudo para sua empresa, em um só lugar."
    notice={notice}
    signupHelp="Esta instalação já possui uma empresa. Novos operadores são vinculados pelo administrador; o cadastro comercial será oferecido no site."
    recoveryHelp="A recuperação da senha local exige verificação do dono ou administrador desta instalação. O fluxo automático ainda não está disponível."
    errorMessage={localErrorMessage}
    onSubmit={async (id, password) => { await login(id, password); navigate('/local/home', { replace: true }); }}
    secondary={<Link to="/login">Área online · acesso por e-mail</Link>}
  />;
}
