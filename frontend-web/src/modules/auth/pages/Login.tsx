import { Link, useNavigate } from 'react-router-dom';
import SignInCard from '../../../shared/brand/SignInCard';
import { useAuthStore } from '../../../core/auth/useAuthStore';

function onlineErrorMessage(failure: unknown): string {
  const status = (failure as { response?: { status?: number } })?.response?.status;
  if (status === 429) return 'Muitas tentativas. Aguarde um minuto antes de tentar novamente.';
  if (status === 401) return 'E-mail ou senha incorretos.';
  return 'Não foi possível conectar ao servidor. Tente novamente mais tarde.';
}

export default function Login() {
  const login = useAuthStore((state) => state.login);
  const navigate = useNavigate();
  return <SignInCard
    credentialLabel="E-mail"
    credentialPlaceholder="voce@empresa.com.br"
    credentialType="email"
    subtitle="Tudo para sua empresa, em um só lugar."
    signupHelp="O cadastro comercial pela web ainda não está disponível nesta versão."
    recoveryHelp="A recuperação por e-mail ainda não está disponível nesta versão. Procure o administrador da sua empresa."
    errorMessage={onlineErrorMessage}
    onSubmit={async (email, password) => { await login(email, password); navigate('/', { replace: true }); }}
    secondary={<>Nesta instalação? <Link to="/local/login">Entrar no acesso local</Link></>}
  />;
}
