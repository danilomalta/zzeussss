import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { ArrowRight, Chrome, Facebook } from 'lucide-react';
import { useAuth } from '../../context/AuthContext';
import { useLanguage } from '../../context/LanguageContext';
import api from '../../services/api';
import ForgotPasswordModal from '../../components/common/ForgotPasswordModal';

const Login: React.FC = () => {
    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [loading, setLoading] = useState(false);
    const [showForgotPassword, setShowForgotPassword] = useState(false);
    const navigate = useNavigate();
    const { login } = useAuth();
    const { t } = useLanguage();

    const handleLogin = async (e: React.FormEvent) => {
        e.preventDefault();
        setLoading(true);
        try {
            const res = await api.post('/auth/login', { email, password });
            login(res.data.token, res.data.user);
            
            // Verifica se é Electron ou Browser e redireciona adequadamente
            const isElectron = navigator.userAgent.toLowerCase().indexOf(' electron/') > -1;
            if (isElectron) {
                navigate('/dashboard');
            } else {
                navigate('/web-panel');
            }
        } catch (err: any) {
            alert('Login Failed: ' + (err.response?.data?.error || 'Unknown Error'));
        } finally {
            setLoading(false);
        }
    };

    const handleSocialLogin = (provider: string) => {
        console.log(`Initiating ${provider} OAuth flow...`);
        alert(`Funcionalidade requer configuração de Chaves API no Backend (${provider})`);
    };

    return (
        <>
            <div className="card-wrapper">
                <div className="glass-panel card-face">
                    <div style={{ marginBottom: '28px', textAlign: 'center' }}>
                        <h2 className="holographic-text" style={{ fontSize: '1.6rem', marginBottom: '8px' }}>
                            {t('welcome')}
                        </h2>
                        <p style={{ color: 'var(--text-secondary)', fontSize: '0.85rem' }}>
                            {t('accessTitan')}
                        </p>
                    </div>

                    <div className="social-container">
                        <button 
                            className="social-btn" 
                            onClick={() => handleSocialLogin('Google')}
                            type="button"
                        >
                            <Chrome size={16} /> Google
                        </button>
                        <button 
                            className="social-btn" 
                            onClick={() => handleSocialLogin('Facebook')}
                            type="button"
                        >
                            <Facebook size={16} /> Facebook
                        </button>
                    </div>

                    <div className="divider"><span>OR</span></div>

                    <form onSubmit={handleLogin}>
                        <div className="input-group">
                            <input
                                type="email" 
                                placeholder={t('email')} 
                                className="input-field"
                                value={email} 
                                onChange={e => setEmail(e.target.value)}
                                required
                            />
                        </div>
                        <div className="input-group">
                            <input
                                type="password" 
                                placeholder={t('password')} 
                                className="input-field"
                                value={password} 
                                onChange={e => setPassword(e.target.value)}
                                required
                            />
                        </div>
                        <button 
                            type="submit" 
                            className="primary-btn" 
                            disabled={loading}
                        >
                            {loading ? t('loading') : t('enterSystem')}
                        </button>
                    </form>

                    <button 
                        onClick={() => setShowForgotPassword(true)} 
                        className="link-btn" 
                        style={{ marginTop: '12px', fontSize: '0.8rem' }}
                        type="button"
                    >
                        {t('forgotPassword')}
                    </button>
                    <button 
                        onClick={() => navigate('/register')} 
                        className="link-btn" 
                        style={{ marginTop: '16px', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '5px' }}
                        type="button"
                    >
                        {t('createAccount')} <ArrowRight size={14} />
                    </button>
                </div>
            </div>

            {/* Modal de Recuperar Senha */}
            <ForgotPasswordModal 
                isOpen={showForgotPassword} 
                onClose={() => setShowForgotPassword(false)} 
            />
        </>
    );
};

export default Login;
