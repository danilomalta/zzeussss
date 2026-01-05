import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useLanguage } from '../../context/LanguageContext';
import LanguageSwitcher from '../../components/features/LanguageSwitcher';
import api from '../../services/api';

// Função para máscara de telefone
const formatPhone = (value: string): string => {
    const numbers = value.replace(/\D/g, '');
    if (numbers.length <= 10) {
        return numbers.replace(/(\d{2})(\d{4})(\d{0,4})/, (_, d1, d2, d3) => {
            if (d3) return `(${d1}) ${d2}-${d3}`;
            if (d2) return `(${d1}) ${d2}`;
            if (d1) return `(${d1}`;
            return '';
        });
    }
    return numbers.replace(/(\d{2})(\d{5})(\d{0,4})/, (_, d1, d2, d3) => {
        if (d3) return `(${d1}) ${d2}-${d3}`;
        if (d2) return `(${d1}) ${d2}`;
        if (d1) return `(${d1}`;
        return '';
    });
};

const RegisterWizard = () => {
    const [step, setStep] = useState(1);
    const [loading, setLoading] = useState(false);
    const navigate = useNavigate();
    const { lang } = useLanguage();
    
    const [formData, setFormData] = useState({
        plan: '',
        cnpj: '',
        cpf: '',
        email: '',
        companyName: '',
        contact: '',
        serverType: '',
        size: '',
        sector: '',
        infra: '',
    });

    const nextStep = () => {
        if (step < 4) setStep(s => s + 1);
    };

    const prevStep = () => {
        if (step > 1) setStep(s => s - 1);
    };

    const handlePlanSelect = (plan: string) => {
        setFormData({ ...formData, plan });
    };

    const handleInfraSelect = (infra: string) => {
        setFormData({ ...formData, infra });
    };

    const handleInputChange = (field: string, value: string) => {
        if (field === 'contact') {
            const formatted = formatPhone(value);
            setFormData({ ...formData, [field]: formatted });
        } else {
            setFormData({ ...formData, [field]: value });
        }
    };

    const handleFinalize = async () => {
        setLoading(true);
        try {
            const payload = {
                plan: formData.plan,
                company: {
                    cnpj: formData.cnpj || formData.cpf,
                    name: formData.companyName,
                    email: formData.email,
                    contact: formData.contact.replace(/\D/g, ''),
                    size: formData.size,
                    sector: formData.sector,
                },
                infrastructure: {
                    type: formData.infra,
                    serverType: formData.serverType,
                },
            };

            const response = await api.post('/auth/register', payload);
            
            if (response.data.success) {
                alert('Cadastro realizado com sucesso! Redirecionando...');
                const isElectron = navigator.userAgent.toLowerCase().indexOf(' electron/') > -1;
                navigate(isElectron ? '/dashboard' : '/web-panel');
            }
        } catch (error: any) {
            alert('Erro ao finalizar cadastro: ' + (error.response?.data?.error || 'Erro desconhecido'));
        } finally {
            setLoading(false);
        }
    };

    const plans = [
        { id: 'start', name: 'Start', description: 'Recursos essenciais para pequenos negócios' },
        { id: 'pro', name: 'Pro', description: 'Ideal para empresas em crescimento' },
        { id: 'corp', name: 'Corp', description: 'Soluções corporativas completas' },
        { id: 'titan', name: 'Titan', description: 'Máxima performance e recursos' },
        { id: 'elite', name: 'Elite', description: 'Solução premium personalizada' },
    ];

    const companySizes = [
        { value: 'micro', label: 'Microempresa (1-9 funcionários)' },
        { value: 'small', label: 'Pequena (10-49 funcionários)' },
        { value: 'medium', label: 'Média (50-249 funcionários)' },
        { value: 'large', label: 'Grande (250+ funcionários)' },
    ];

    const sectors = [
        { value: 'retail', label: 'Varejo' },
        { value: 'services', label: 'Serviços' },
        { value: 'industry', label: 'Indústria' },
        { value: 'food', label: 'Alimentação' },
        { value: 'health', label: 'Saúde' },
        { value: 'education', label: 'Educação' },
        { value: 'other', label: 'Outro' },
    ];

    const infraOptions = [
        { id: 'cloud', label: 'Nuvem Titan', icon: '☁️', description: 'Hospedagem gerenciada na nuvem' },
        { id: 'local', label: 'Servidor Local', icon: '🖥️', description: 'Instalação no seu servidor' },
        { id: 'hd', label: 'HD Próprio', icon: '💾', description: 'Armazenamento próprio' },
    ];

    // PASSO 1: PLANOS
    const renderStep1 = () => (
        <div className="wizard-content">
            <h2 className="holographic-text" style={{ textAlign: 'center', marginBottom: '40px', fontSize: '2rem' }}>
                Escolha seu Plano
            </h2>
            <div style={{ 
                display: 'grid', 
                gridTemplateColumns: 'repeat(5, 1fr)', 
                gap: '20px',
                marginBottom: '40px'
            }}>
                {plans.map((plan) => (
                    <div
                        key={plan.id}
                        className={`holo-card ${formData.plan === plan.id ? 'selected' : ''}`}
                        onClick={() => handlePlanSelect(plan.id)}
                    >
                        <h3 style={{ 
                            color: formData.plan === plan.id ? 'var(--accent-color)' : 'var(--text-primary)', 
                            marginBottom: '10px',
                            fontSize: '1.2rem'
                        }}>
                            {plan.name}
                        </h3>
                        <p style={{ 
                            fontSize: '0.85rem', 
                            color: 'var(--text-secondary)',
                            lineHeight: '1.4'
                        }}>
                            {plan.description}
                        </p>
                    </div>
                ))}
            </div>
            <div style={{ textAlign: 'center' }}>
                <button 
                    className="titan-btn" 
                    onClick={nextStep} 
                    disabled={!formData.plan}
                    style={{ 
                        opacity: formData.plan ? 1 : 0.4,
                        width: '100%',
                        height: '48px',
                        borderRadius: '12px'
                    }}
                >
                    Continuar
                </button>
            </div>
        </div>
    );

    // PASSO 2: DADOS
    const renderStep2 = () => (
        <div className="wizard-content" style={{ maxWidth: '600px', margin: '0 auto' }}>
            <h2 className="holographic-text" style={{ textAlign: 'center', marginBottom: '40px', fontSize: '2rem' }}>
                Dados da Empresa
            </h2>
            <div className="wizard-scrollable">
                <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
                <div>
                    <label style={{ 
                        display: 'block', 
                        marginBottom: '8px', 
                        color: 'var(--text-secondary)',
                        fontSize: '0.9rem'
                    }}>
                        CNPJ ou CPF
                    </label>
                    <input
                        type="text"
                        placeholder="CNPJ/CPF (Busca Automática)"
                        className="titan-input"
                        value={formData.cnpj || formData.cpf}
                        onChange={(e) => {
                            const value = e.target.value;
                            if (value.length <= 14) {
                                handleInputChange('cpf', value);
                                handleInputChange('cnpj', '');
                            } else {
                                handleInputChange('cnpj', value);
                                handleInputChange('cpf', '');
                            }
                        }}
                    />
                </div>
                <div>
                    <label style={{ 
                        display: 'block', 
                        marginBottom: '8px', 
                        color: 'var(--text-secondary)',
                        fontSize: '0.9rem'
                    }}>
                        Nome da Empresa
                    </label>
                    <input
                        type="text"
                        placeholder="Nome da Empresa"
                        className="titan-input"
                        value={formData.companyName}
                        onChange={(e) => handleInputChange('companyName', e.target.value)}
                    />
                </div>
                <div>
                    <label style={{ 
                        display: 'block', 
                        marginBottom: '8px', 
                        color: 'var(--text-secondary)',
                        fontSize: '0.9rem'
                    }}>
                        Email
                    </label>
                    <input
                        type="email"
                        placeholder="email@empresa.com"
                        className="titan-input"
                        value={formData.email}
                        onChange={(e) => handleInputChange('email', e.target.value)}
                    />
                </div>
                <div>
                    <label style={{ 
                        display: 'block', 
                        marginBottom: '8px', 
                        color: 'var(--text-secondary)',
                        fontSize: '0.9rem'
                    }}>
                        Tamanho da Empresa
                    </label>
                    <select
                        className="titan-input"
                        value={formData.size}
                        onChange={(e) => handleInputChange('size', e.target.value)}
                    >
                        <option value="">Selecione o Tamanho</option>
                        {companySizes.map((size) => (
                            <option key={size.value} value={size.value}>
                                {size.label}
                            </option>
                        ))}
                    </select>
                </div>
                <div>
                    <label style={{ 
                        display: 'block', 
                        marginBottom: '8px', 
                        color: 'var(--text-secondary)',
                        fontSize: '0.9rem'
                    }}>
                        Setor
                    </label>
                    <select
                        className="titan-input"
                        value={formData.sector}
                        onChange={(e) => handleInputChange('sector', e.target.value)}
                    >
                        <option value="">Selecione o Setor</option>
                        {sectors.map((sector) => (
                            <option key={sector.value} value={sector.value}>
                                {sector.label}
                            </option>
                        ))}
                    </select>
                </div>
                <div>
                    <label style={{ 
                        display: 'block', 
                        marginBottom: '8px', 
                        color: 'var(--text-secondary)',
                        fontSize: '0.9rem'
                    }}>
                        Contato/WhatsApp *
                    </label>
                    <input
                        type="tel"
                        placeholder="(00) 00000-0000"
                        className="titan-input"
                        value={formData.contact}
                        onChange={(e) => handleInputChange('contact', e.target.value)}
                        maxLength={15}
                    />
                </div>
                <div>
                    <label style={{ 
                        display: 'block', 
                        marginBottom: '8px', 
                        color: 'var(--text-secondary)',
                        fontSize: '0.9rem'
                    }}>
                        Tipo de Servidor *
                    </label>
                    <select
                        className="titan-input"
                        value={formData.serverType}
                        onChange={(e) => handleInputChange('serverType', e.target.value)}
                    >
                        <option value="">Selecione o Tipo</option>
                        <option value="cloud">Nuvem Titan</option>
                        <option value="local">Servidor Local</option>
                        <option value="hybrid">Híbrido</option>
                    </select>
                </div>
                </div>
            </div>
            <div style={{ 
                display: 'flex', 
                justifyContent: 'space-between', 
                marginTop: '24px',
                gap: '20px'
            }}>
                <button 
                    className="titan-btn" 
                    style={{ 
                        background: 'transparent', 
                        border: '1px solid var(--glass-border)', 
                        color: 'white',
                        flex: 1
                    }} 
                    onClick={prevStep}
                >
                    Voltar
                </button>
                <button 
                    className="titan-btn" 
                    onClick={nextStep}
                    style={{ flex: 1, height: '48px', borderRadius: '12px' }}
                    disabled={!formData.companyName || !formData.email || !formData.size || !formData.sector || !formData.contact || !formData.serverType}
                >
                    Continuar
                </button>
            </div>
        </div>
    );

    // PASSO 3: INFRAESTRUTURA
    const renderStep3 = () => (
        <div className="wizard-content">
            <h2 className="holographic-text" style={{ textAlign: 'center', marginBottom: '40px', fontSize: '2rem' }}>
                Infraestrutura
            </h2>
            <div style={{ 
                display: 'grid', 
                gridTemplateColumns: 'repeat(3, 1fr)', 
                gap: '30px',
                marginBottom: '40px'
            }}>
                {infraOptions.map((infra) => (
                    <div
                        key={infra.id}
                        className={`holo-card ${formData.infra === infra.id ? 'selected' : ''}`}
                        onClick={() => handleInfraSelect(infra.id)}
                        style={{ 
                            textAlign: 'center', 
                            padding: '40px',
                            minHeight: '250px',
                            display: 'flex',
                            flexDirection: 'column',
                            justifyContent: 'center'
                        }}
                    >
                        <div style={{ fontSize: '4rem', marginBottom: '20px' }}>{infra.icon}</div>
                        <h3 style={{ 
                            color: formData.infra === infra.id ? 'var(--accent-color)' : 'var(--text-primary)',
                            marginBottom: '10px',
                            fontSize: '1.3rem'
                        }}>
                            {infra.label}
                        </h3>
                        <p style={{ 
                            color: 'var(--text-secondary)',
                            fontSize: '0.9rem',
                            marginTop: '10px'
                        }}>
                            {infra.description}
                        </p>
                    </div>
                ))}
            </div>
            <div style={{ 
                display: 'flex', 
                justifyContent: 'space-between',
                gap: '20px'
            }}>
                <button 
                    className="titan-btn" 
                    style={{ 
                        background: 'transparent', 
                        border: '1px solid var(--glass-border)', 
                        color: 'white',
                        flex: 1
                    }} 
                    onClick={prevStep}
                >
                    Voltar
                </button>
                <button 
                    className="titan-btn" 
                    onClick={nextStep} 
                    disabled={!formData.infra}
                    style={{ 
                        opacity: formData.infra ? 1 : 0.4,
                        flex: 1,
                        height: '48px',
                        borderRadius: '12px'
                    }}
                >
                    Continuar
                </button>
            </div>
        </div>
    );

    // PASSO 4: PAGAMENTO
    const renderStep4 = () => {
        const planPrices: { [key: string]: number } = {
            start: 99.90,
            pro: 199.90,
            corp: 399.90,
            titan: 799.90,
            elite: 1499.90,
        };

        const infraPrices: { [key: string]: number } = {
            cloud: 50.00,
            local: 0,
            hd: 0,
        };

        const planPrice = planPrices[formData.plan] || 0;
        const infraPrice = infraPrices[formData.infra] || 0;
        const total = planPrice + infraPrice;

        return (
            <div className="wizard-content" style={{ maxWidth: '500px', margin: '0 auto' }}>
                <h2 className="holographic-text" style={{ textAlign: 'center', marginBottom: '40px', fontSize: '2rem' }}>
                    Pagamento
                </h2>
                <div className="glass-panel" style={{ padding: '30px', marginBottom: '30px' }}>
                    <h3 style={{ 
                        color: 'var(--accent-color)', 
                        marginBottom: '20px',
                        fontSize: '1.2rem'
                    }}>
                        Resumo da Assinatura
                    </h3>
                    <div style={{ 
                        marginBottom: '20px', 
                        borderBottom: '1px solid var(--glass-border)', 
                        paddingBottom: '20px' 
                    }}>
                        <div style={{ 
                            display: 'flex', 
                            justifyContent: 'space-between', 
                            marginBottom: '10px' 
                        }}>
                            <span style={{ color: 'var(--text-secondary)' }}>Plano {formData.plan.toUpperCase()}</span>
                            <span style={{ fontWeight: 'bold' }}>R$ {planPrice.toFixed(2)}/mês</span>
                        </div>
                        <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                            <span style={{ color: 'var(--text-secondary)' }}>
                                Infraestrutura: {infraOptions.find(i => i.id === formData.infra)?.label}
                            </span>
                            <span style={{ fontWeight: 'bold' }}>
                                {infraPrice > 0 ? `R$ ${infraPrice.toFixed(2)}/mês` : 'Incluído'}
                            </span>
                        </div>
                    </div>
                    <div style={{ 
                        display: 'flex', 
                        justifyContent: 'space-between', 
                        fontSize: '1.3rem', 
                        fontWeight: 'bold',
                        marginBottom: '30px',
                        paddingTop: '20px',
                        borderTop: '1px solid var(--glass-border)'
                    }}>
                        <span>Total</span>
                        <span style={{ color: 'var(--accent-color)' }}>R$ {total.toFixed(2)}/mês</span>
                    </div>

                    <h4 style={{ 
                        color: 'var(--text-primary)', 
                        marginBottom: '15px',
                        fontSize: '1rem'
                    }}>
                        Dados do Cartão
                    </h4>
                    <input 
                        type="text" 
                        placeholder="Número do Cartão" 
                        className="titan-input" 
                        style={{ marginBottom: '15px' }} 
                    />
                    <div style={{ display: 'flex', gap: '15px', marginBottom: '15px' }}>
                        <input 
                            type="text" 
                            placeholder="Validade (MM/AA)" 
                            className="titan-input" 
                        />
                        <input 
                            type="text" 
                            placeholder="CVV" 
                            className="titan-input" 
                        />
                    </div>
                    <input 
                        type="text" 
                        placeholder="Nome no Cartão" 
                        className="titan-input" 
                        style={{ marginBottom: '30px' }} 
                    />

                    <button 
                        className="titan-btn final-btn" 
                        style={{ width: '100%', height: '48px', borderRadius: '12px' }} 
                        onClick={handleFinalize}
                        disabled={loading}
                    >
                        {loading ? 'Processando...' : 'FINALIZAR CADASTRO'}
                    </button>
                </div>
                <div style={{ display: 'flex', justifyContent: 'flex-start' }}>
                    <button 
                        className="titan-btn" 
                        style={{ 
                            background: 'transparent', 
                            border: '1px solid var(--glass-border)', 
                            color: 'white' 
                        }} 
                        onClick={prevStep}
                    >
                        Voltar
                    </button>
                </div>
            </div>
        );
    };

    return (
        <div style={{ 
            position: 'relative', 
            width: '100vw', 
            height: '100vh', 
            overflow: 'hidden',
            background: 'var(--bg-app)'
        }}>
            {/* Barra de Progresso no Topo (FIXA) */}
            <div className="wizard-progress-top">
                <div 
                    className="wizard-progress-fill-top"
                    style={{ width: `${(step / 4) * 100}%` }}
                />
            </div>


            {/* Back to Login Button */}
            <div className="top-left-controls">
                <button 
                    className="icon-btn" 
                    onClick={() => navigate('/')}
                    aria-label="Voltar para login"
                    title="Voltar para login"
                >
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                        <path d="M19 12H5M12 19l-7-7 7-7"/>
                    </svg>
                </button>
            </div>

            {/* Language Switcher (Bottom Right) */}
            <LanguageSwitcher />

            <div className="wizard-container">

                {/* Indicadores de Passo */}
                <div className="wizard-steps-indicator">
                    {[1, 2, 3, 4].map((s) => (
                        <div 
                            key={s} 
                            className={`step-indicator ${step >= s ? 'active' : ''}`} 
                        />
                    ))}
                </div>

                {step === 1 && renderStep1()}
                {step === 2 && renderStep2()}
                {step === 3 && renderStep3()}
                {step === 4 && renderStep4()}
            </div>
        </div>
    );
};

export default RegisterWizard;
