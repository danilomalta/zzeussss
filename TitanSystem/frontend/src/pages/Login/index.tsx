import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { ArrowRight, Chrome, Facebook, CreditCard } from 'lucide-react';
import { useAuth } from '../../context/AuthContext';
import api from '../../services/api';

const RegisterWizard = ({ onBack }: { onBack: () => void }) => {
    const [step, setStep] = useState(1);
    const [formData, setFormData] = useState({ company: '', email: '', password: '', plan: 'pro' });
    const [loading, setLoading] = useState(false);

    const handleRegister = async () => {
        setLoading(true);
        try {
            // Mock API call for registration
            // await api.post('/auth/register', { ...formData });
            setTimeout(() => {
                alert('Registration Successful! Please Login.');
                onBack();
            }, 1500);
        } catch (err) {
            alert('Registration Failed');
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="glass card-face card-back">
            <div className="wizard-steps">
                <div className={`step-dot ${step >= 1 ? 'active' : ''}`} />
                <div className={`step-dot ${step >= 2 ? 'active' : ''}`} />
                <div className={`step-dot ${step >= 3 ? 'active' : ''}`} />
            </div>

            <h2 style={{ fontSize: '1.5rem', marginBottom: '20px', textAlign: 'center' }}>
                {step === 1 ? 'Account Data' : step === 2 ? 'Choose Plan' : 'Payment'}
            </h2>

            {step === 1 && (
                <>
                    <div className="input-group"><input type="text" placeholder="Company Name" className="input-field" value={formData.company} onChange={e => setFormData({ ...formData, company: e.target.value })} /></div>
                    <div className="input-group"><input type="email" placeholder="Email" className="input-field" value={formData.email} onChange={e => setFormData({ ...formData, email: e.target.value })} /></div>
                    <div className="input-group"><input type="password" placeholder="Password" className="input-field" value={formData.password} onChange={e => setFormData({ ...formData, password: e.target.value })} /></div>
                    <button className="primary-btn" onClick={() => setStep(2)}>Next</button>
                </>
            )}

            {step === 2 && (
                <>
                    <div className={`plan-card ${formData.plan === 'start' ? 'selected' : ''}`} onClick={() => setFormData({ ...formData, plan: 'start' })}>
                        <strong>Start</strong> - $29/mo
                    </div>
                    <div className={`plan-card ${formData.plan === 'pro' ? 'selected' : ''}`} onClick={() => setFormData({ ...formData, plan: 'pro' })}>
                        <strong>Pro</strong> - $99/mo
                    </div>
                    <div className={`plan-card ${formData.plan === 'corp' ? 'selected' : ''}`} onClick={() => setFormData({ ...formData, plan: 'corp' })}>
                        <strong>Corp</strong> - $299/mo
                    </div>
                    <div style={{ display: 'flex', gap: '10px' }}>
                        <button className="glass-button" style={{ flex: 1 }} onClick={() => setStep(1)}>Back</button>
                        <button className="primary-btn" style={{ flex: 1 }} onClick={() => setStep(3)}>Next</button>
                    </div>
                </>
            )}

            {step === 3 && (
                <>
                    <div style={{ textAlign: 'center', margin: '20px 0' }}>
                        <CreditCard size={48} style={{ marginBottom: '10px', color: 'var(--accent-color)' }} />
                        <p>Secure Payment Gateway</p>
                    </div>
                    <div className="input-group"><input type="text" placeholder="Card Number" className="input-field" /></div>
                    <div style={{ display: 'flex', gap: '10px' }}>
                        <input type="text" placeholder="MM/YY" className="input-field" />
                        <input type="text" placeholder="CVC" className="input-field" />
                    </div>
                    <div style={{ display: 'flex', gap: '10px', marginTop: '20px' }}>
                        <button className="glass-button" style={{ flex: 1 }} onClick={() => setStep(2)}>Back</button>
                        <button className="primary-btn" style={{ flex: 1 }} onClick={handleRegister} disabled={loading}>
                            {loading ? 'Processing...' : 'Finish'}
                        </button>
                    </div>
                </>
            )}

            <button onClick={onBack} className="link-btn" style={{ marginTop: 'auto' }}>Back to Login</button>
        </div>
    );
};

const Login: React.FC = () => {
    const [isFlipped, setIsFlipped] = useState(false);
    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [loading, setLoading] = useState(false);
    const navigate = useNavigate();
    const { login } = useAuth();

    const handleLogin = async (e: React.FormEvent) => {
        e.preventDefault();
        setLoading(true);
        try {
            const res = await api.post('/auth/login', { email, password });
            login(res.data.token, res.data.user);
            navigate('/dashboard');
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
        <div className={`card-wrapper ${isFlipped ? 'flipped' : ''}`}>
            {/* FRONT: LOGIN */}
            <div className="glass card-face card-front">
                <div style={{ marginBottom: '20px' }}>
                    <h2 style={{ fontSize: '1.5rem', marginBottom: '5px' }}>Welcome</h2>
                    <p style={{ color: 'var(--text-secondary)', fontSize: '0.9rem' }}>Access Titan System</p>
                </div>

                <div className="social-container">
                    <button className="social-btn" onClick={() => handleSocialLogin('Google')}>
                        <Chrome size={16} /> Google
                    </button>
                    <button className="social-btn" onClick={() => handleSocialLogin('Facebook')}>
                        <Facebook size={16} /> Facebook
                    </button>
                </div>

                <div className="divider"><span>OR</span></div>

                <form onSubmit={handleLogin}>
                    <div className="input-group">
                        <input
                            type="email" placeholder="Email" className="input-field"
                            value={email} onChange={e => setEmail(e.target.value)}
                        />
                    </div>
                    <div className="input-group">
                        <input
                            type="password" placeholder="Password" className="input-field"
                            value={password} onChange={e => setPassword(e.target.value)}
                        />
                    </div>
                    <button type="submit" className="primary-btn" disabled={loading}>
                        {loading ? '...' : 'Enter System'}
                    </button>
                </form>

                <button onClick={() => navigate('/forgot-password')} className="link-btn" style={{ marginTop: '10px', fontSize: '0.8rem' }}>
                    Forgot Password?
                </button>
                <button onClick={() => setIsFlipped(true)} className="link-btn" style={{ marginTop: '20px' }}>
                    Create Account <ArrowRight size={14} style={{ display: 'inline', marginLeft: '4px' }} />
                </button>
            </div>

            {/* BACK: REGISTER WIZARD */}
            <RegisterWizard onBack={() => setIsFlipped(false)} />
        </div>
    );
};

export default Login;
