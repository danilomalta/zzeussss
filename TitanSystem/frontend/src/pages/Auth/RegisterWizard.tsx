import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { CreditCard, Server, Building, CheckCircle } from 'lucide-react';
import api from '../../services/api';

const RegisterWizard = () => {
    const navigate = useNavigate();
    const [step, setStep] = useState(1);
    const [loading, setLoading] = useState(false);
    const [formData, setFormData] = useState({
        // Step 1: Plan
        plan: 'pro',
        // Step 2: Company & User
        companyName: '',
        email: '',
        password: '',
        cnpj: '',
        whatsapp: '',
        businessSize: 'small',
        sector: 'retail',
        // Step 3: Infrastructure
        storageType: 'cloud',
        // Step 4: Payment
        cardNumber: '',
        cardExpiry: '',
        cardCvc: ''
    });

    const handleChange = (e: any) => {
        setFormData({ ...formData, [e.target.name]: e.target.value });
    };

    const handleFinish = async () => {
        setLoading(true);
        try {
            await api.post('/auth/register', formData);
            alert('Cadastro realizado com sucesso! Faça login.');
            navigate('/');
        } catch (err: any) {
            alert('Erro ao cadastrar: ' + (err.response?.data?.error || 'Erro desconhecido'));
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="wizard-container">
            {/* STEPS INDICATOR */}
            <div className="wizard-steps-indicator">
                {[1, 2, 3, 4].map(s => (
                    <div key={s} className={`step-indicator ${step >= s ? 'active' : ''}`} />
                ))}
            </div>

            <div className="glass card-face" style={{ maxWidth: '800px', margin: '0 auto', width: '100%' }}>
                <h2 className="holographic-text" style={{ textAlign: 'center', marginBottom: '30px', fontSize: '1.5rem' }}>
                    {step === 1 && 'Escolha seu Plano'}
                    {step === 2 && 'Dados da Empresa'}
                    {step === 3 && 'Infraestrutura'}
                    {step === 4 && 'Pagamento'}
                </h2>

                <div style={{ maxHeight: '60vh', overflowY: 'auto', paddingRight: '10px' }}>

                    {/* STEP 1: PLANS */}
                    {step === 1 && (
                        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '20px' }}>
                            {['start', 'pro', 'corp'].map(p => (
                                <div
                                    key={p}
                                    className={`holo-card ${formData.plan === p ? 'selected' : ''}`}
                                    onClick={() => setFormData({ ...formData, plan: p })}
                                    style={{ textAlign: 'center' }}
                                >
                                    <h3 style={{ textTransform: 'uppercase', color: 'var(--accent-color)' }}>{p}</h3>
                                    <p style={{ margin: '10px 0', fontSize: '0.9rem', color: 'var(--text-secondary)' }}>
                                        {p === 'start' ? 'Para iniciantes' : p === 'pro' ? 'Para crescimento' : 'Para grandes empresas'}
                                    </p>
                                </div>
                            ))}
                        </div>
                    )}

                    {/* STEP 2: COMPANY DATA */}
                    {step === 2 && (
                        <div style={{ display: 'grid', gap: '15px' }}>
                            <div className="input-group">
                                <label>Nome da Empresa</label>
                                <input name="companyName" value={formData.companyName} onChange={handleChange} placeholder="Ex: Titan Corp" />
                            </div>
                            <div className="input-group">
                                <label>Email Administrativo</label>
                                <input name="email" value={formData.email} onChange={handleChange} placeholder="admin@titan.com" />
                            </div>
                            <div className="input-group">
                                <label>Senha Mestra</label>
                                <input type="password" name="password" value={formData.password} onChange={handleChange} placeholder="******" />
                            </div>
                            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '15px' }}>
                                <div>
                                    <label>CNPJ</label>
                                    <input name="cnpj" value={formData.cnpj} onChange={handleChange} placeholder="00.000.000/0001-00" />
                                </div>
                                <div>
                                    <label>WhatsApp / Contato</label>
                                    <input name="whatsapp" value={formData.whatsapp} onChange={handleChange} placeholder="(11) 99999-9999" />
                                </div>
                            </div>
                            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '15px' }}>
                                <div>
                                    <label>Tamanho</label>
                                    <select name="businessSize" value={formData.businessSize} onChange={handleChange}>
                                        <option value="mei">MEI</option>
                                        <option value="micro">Micro</option>
                                        <option value="small">Pequena</option>
                                        <option value="medium">Média</option>
                                        <option value="large">Grande</option>
                                    </select>
                                </div>
                                <div>
                                    <label>Setor</label>
                                    <select name="sector" value={formData.sector} onChange={handleChange}>
                                        <option value="retail">Varejo</option>
                                        <option value="services">Serviços</option>
                                        <option value="tech">Tecnologia</option>
                                        <option value="industry">Indústria</option>
                                    </select>
                                </div>
                            </div>
                        </div>
                    )}

                    {/* STEP 3: INFRASTRUCTURE */}
                    {step === 3 && (
                        <div>
                            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '20px', marginBottom: '20px' }}>
                                <div
                                    className={`holo-card ${formData.storageType === 'cloud' ? 'selected' : ''}`}
                                    onClick={() => setFormData({ ...formData, storageType: 'cloud' })}
                                >
                                    <Server size={32} style={{ marginBottom: '10px' }} />
                                    <h4>Titan Cloud</h4>
                                    <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>100% Nuvem Segura</p>
                                </div>
                                <div
                                    className={`holo-card ${formData.storageType === 'local' ? 'selected' : ''}`}
                                    onClick={() => setFormData({ ...formData, storageType: 'local' })}
                                >
                                    <Building size={32} style={{ marginBottom: '10px' }} />
                                    <h4>Local Server</h4>
                                    <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>Servidor Próprio</p>
                                </div>
                                <div
                                    className={`holo-card ${formData.storageType === 'hybrid' ? 'selected' : ''}`}
                                    onClick={() => setFormData({ ...formData, storageType: 'hybrid' })}
                                >
                                    <CheckCircle size={32} style={{ marginBottom: '10px' }} />
                                    <h4>Híbrido</h4>
                                    <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>Sync Automático</p>
                                </div>
                            </div>
                        </div>
                    )}

                    {/* STEP 4: PAYMENT */}
                    {step === 4 && (
                        <div>
                            <div style={{ textAlign: 'center', marginBottom: '20px' }}>
                                <CreditCard size={48} style={{ color: 'var(--accent-color)' }} />
                                <p style={{ marginTop: '10px' }}>Gateway Seguro AES-256</p>
                            </div>
                            <div className="input-group">
                                <label>Número do Cartão</label>
                                <input name="cardNumber" value={formData.cardNumber} onChange={handleChange} placeholder="0000 0000 0000 0000" />
                            </div>
                            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '15px' }}>
                                <div>
                                    <label>Validade</label>
                                    <input name="cardExpiry" value={formData.cardExpiry} onChange={handleChange} placeholder="MM/AA" />
                                </div>
                                <div>
                                    <label>CVC</label>
                                    <input name="cardCvc" value={formData.cardCvc} onChange={handleChange} placeholder="123" />
                                </div>
                            </div>
                        </div>
                    )}

                </div>

                {/* NAVIGATION BUTTONS */}
                <div style={{ display: 'flex', gap: '15px', marginTop: '30px' }}>
                    {step > 1 && (
                        <button className="btn btn-ghost" onClick={() => setStep(step - 1)}>
                            Voltar
                        </button>
                    )}

                    {step < 4 ? (
                        <button className="btn btn-primary" onClick={() => setStep(step + 1)}>
                            Próximo
                        </button>
                    ) : (
                        <button className="btn btn-primary" onClick={handleFinish} disabled={loading}>
                            {loading ? 'Processando...' : 'FINALIZAR CADASTRO'}
                        </button>
                    )}
                </div>

            </div>
        </div>
    );
};

export default RegisterWizard;
