import React, { useState, useEffect } from 'react';
import { Routes, Route, useNavigate, Navigate, useLocation } from 'react-router-dom';
import {
  Menu, Moon, Sun, ArrowRight, LogOut, Users, Settings, LayoutDashboard, Briefcase,
  Globe, Facebook, Chrome, ChevronDown, X, CheckCircle, CreditCard, Shield, Home
} from 'lucide-react';
import { useAuth, AuthProvider } from './context/AuthContext';
import api from './services/api';

// --- TRANSLATIONS ---
const translations: any = {
  en: {
    flag: "🇺🇸 EN",
    welcome: "Welcome",
    access: "Access Titan System",
    email: "Email Address",
    password: "Password",
    enter: "Enter System",
    create: "Create Account",
    join: "Join Titan",
    company: "Company Name",
    register: "Register",
    back: "Back to Login",
    google: "Google",
    facebook: "Facebook",
    forgot: "Forgot Password?",
    steps: ["Account Data", "Choose Plan", "Payment"]
  },
  pt: {
    flag: "🇧🇷 PT",
    welcome: "Bem-vindo",
    access: "Acesse o Sistema Titan",
    email: "Endereço de Email",
    password: "Senha",
    enter: "Entrar no Sistema",
    create: "Criar Conta",
    join: "Junte-se ao Titan",
    company: "Nome da Empresa",
    register: "Registrar",
    back: "Voltar ao Login",
    google: "Google",
    facebook: "Facebook",
    forgot: "Esqueceu a Senha?",
    steps: ["Dados da Conta", "Escolher Plano", "Pagamento"]
  },
  fr: {
    flag: "🇫🇷 FR",
    welcome: "Bienvenue",
    access: "Accéder au Système Titan",
    email: "Adresse Email",
    password: "Mot de passe",
    enter: "Entrer dans le système",
    create: "Créer un compte",
    join: "Rejoindre Titan",
    company: "Nom de l'entreprise",
    register: "S'inscrire",
    back: "Retour à la connexion",
    google: "Google",
    facebook: "Facebook",
    forgot: "Mot de passe oublié?",
    steps: ["Données du compte", "Choisir un plan", "Paiement"]
  },
  cn: {
    flag: "🇨🇳 CN",
    welcome: "欢迎",
    access: "访问 Titan 系统",
    email: "电子邮件地址",
    password: "密码",
    enter: "进入系统",
    create: "创建账户",
    join: "加入 Titan",
    company: "公司名称",
    register: "注册",
    back: "返回登录",
    google: "Google",
    facebook: "Facebook",
    forgot: "忘记密码？",
    steps: ["账户数据", "选择计划", "付款"]
  },
  jp: {
    flag: "🇯🇵 JP",
    welcome: "ようこそ",
    access: "Titanシステムへアクセス",
    email: "メールアドレス",
    password: "パスワード",
    enter: "システムに入る",
    create: "アカウント作成",
    join: "Titanに参加",
    company: "会社名",
    register: "登録",
    back: "ログインに戻る",
    google: "Google",
    facebook: "Facebook",
    forgot: "パスワードをお忘れですか？",
    steps: ["アカウントデータ", "プランの選択", "支払い"]
  },
  ar: {
    flag: "🇸🇦 AR",
    welcome: "أهلاً بك",
    access: "الوصول إلى نظام تيتان",
    email: "عنوان البريد الإلكتروني",
    password: "كلمة المرور",
    enter: "دخول النظام",
    create: "إنشاء حساب",
    join: "انضم إلى تيتان",
    company: "اسم الشركة",
    register: "تسجيل",
    back: "العودة لتسجيل الدخول",
    google: "Google",
    facebook: "Facebook",
    forgot: "هل نسيت كلمة المرور؟",
    steps: ["بيانات الحساب", "اختر الخطة", "الدفع"]
  }
};

// --- CSS STYLES (INJECTED) ---
const cssStyles = `
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap');

:root {
  --bg-color: #020202;
  --glass-bg: rgba(255, 255, 255, 0.03);
  --glass-border: rgba(255, 255, 255, 0.08);
  --glass-highlight: rgba(255, 255, 255, 0.15);
  --text-primary: #ffffff;
  --text-secondary: #a1a1aa;
  --accent-color: #3b82f6;
  --neon-blue: #00f3ff;
}

.light-mode {
  --bg-color: #f8fafc;
  --glass-bg: rgba(0, 0, 0, 0.03);
  --glass-border: rgba(0, 0, 0, 0.08);
  --glass-highlight: rgba(0, 0, 0, 0.05);
  --text-primary: #0f172a;
  --text-secondary: #64748b;
}

* { box-sizing: border-box; margin: 0; padding: 0; }

body {
  background-color: var(--bg-color);
  color: var(--text-primary);
  font-family: 'Inter', sans-serif;
  overflow: hidden;
  height: 100vh;
  width: 100vw;
  transition: background-color 0.3s ease, color 0.3s ease;
}

/* --- UTILS --- */
.glass {
  background: var(--glass-bg);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border: 1px solid var(--glass-border);
  box-shadow: 0 4px 30px rgba(0, 0, 0, 0.1);
}

.glass-button {
  background: rgba(255, 255, 255, 0.05);
  backdrop-filter: blur(10px);
  border: 1px solid var(--glass-border);
  color: var(--text-primary);
  cursor: pointer;
  transition: all 0.3s ease;
  display: flex;
  align-items: center;
  justify-content: center;
}

.glass-button:hover {
  background: rgba(255, 255, 255, 0.1);
  border-color: var(--glass-highlight);
  box-shadow: 0 0 15px var(--glass-bg);
}

/* --- GATES --- */
.gate-container {
  position: fixed;
  top: 0; left: 0; width: 100%; height: 100%;
  z-index: 40;
  pointer-events: none;
  display: flex;
}

.gate {
  height: 100%;
  width: 50.1%; /* 50.1% to fix seam */
  background-color: var(--bg-color);
  position: absolute;
  top: 0;
  transition: transform 0.1s linear, background-color 0.3s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 41;
}

.gate-left { left: 0; }
.gate-right { right: 0; }

.gate-content {
  position: absolute;
  top: 50%; left: 50%;
  transform: translate(-50%, -50%);
  text-align: center;
  z-index: 42;
  transition: opacity 0.3s ease;
  width: 100%;
}

.titan-logo {
  font-size: 6rem;
  font-weight: 800;
  letter-spacing: 1rem;
  background: linear-gradient(to bottom, var(--text-primary), var(--text-secondary));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  margin-bottom: 2rem;
  text-transform: uppercase;
}

.scroll-hint {
  font-size: 0.9rem;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.2rem;
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 0.5; }
  50% { opacity: 1; }
}

/* --- FLOATING UI --- */
.top-left-controls {
  position: fixed; top: 30px; left: 30px; z-index: 9999;
}
.top-right-controls {
  position: fixed; top: 30px; right: 30px; z-index: 9999;
  display: flex; gap: 15px;
}

.icon-btn {
  width: 45px; height: 45px;
  border-radius: 50%;
}

/* --- LANGUAGE SELECTOR --- */
.lang-selector {
  position: relative;
  width: 100px;
}
.lang-btn {
  width: 100%; height: 45px; padding: 0 10px;
  border-radius: 8px;
  justify-content: space-between;
  font-size: 0.9rem; font-weight: 600;
}
.lang-dropdown {
  position: absolute; top: 110%; right: 0;
  width: 100%;
  background: rgba(10, 10, 10, 0.95);
  border: 1px solid var(--glass-border);
  border-radius: 8px;
  overflow: hidden;
  display: none;
}
.lang-selector:hover .lang-dropdown { display: block; }

.lang-option {
  padding: 10px;
  cursor: pointer;
  color: var(--text-secondary);
  transition: all 0.2s;
  font-size: 0.9rem; text-align: center;
}
.lang-option:hover { background: rgba(255,255,255,0.1); color: white; }

/* --- SIDEBAR --- */
.sidebar {
  position: fixed; top: 0; left: 0;
  height: 100%; width: 280px;
  z-index: 10000; /* High Z-Index */
  background: rgba(10, 10, 10, 0.95); /* Dark Background */
  border-right: 1px solid var(--glass-border);
  transform: translateX(-100%);
  transition: transform 0.5s cubic-bezier(0.4, 0, 0.2, 1);
  padding: 40px 20px;
  display: flex; flex-direction: column;
}
.sidebar.open { transform: translateX(0); }

.sidebar-header {
  font-size: 1.5rem; font-weight: bold; margin-bottom: 40px;
  padding-left: 10px; letter-spacing: 2px;
  color: white;
}

.nav-item {
  display: flex; align-items: center; gap: 12px;
  padding: 16px;
  color: #a1a1aa;
  text-decoration: none;
  border-radius: 12px;
  transition: all 0.2s;
  margin-bottom: 5px;
  cursor: pointer;
}
.nav-item:hover, .nav-item.active {
  background: rgba(255,255,255,0.1);
  color: white;
}

/* --- MAIN CONTENT (3D CARD) --- */
.main-stage {
  position: fixed; top: 0; left: 0;
  width: 100%; height: 100%;
  display: flex; align-items: center;
  justify-content: flex-end; /* RIGHT ALIGNMENT */
  padding-right: 10%;
  z-index: 10;
  perspective: 1500px;
}

.card-wrapper {
  width: 380px;
  height: auto;
  min-height: 500px;
  position: relative;
  transform-style: preserve-3d;
  transition: transform 0.8s cubic-bezier(0.175, 0.885, 0.32, 1.275);
}
.card-wrapper.flipped { transform: rotateY(180deg); }

.card-face {
  position: relative;
  width: 100%; height: 100%;
  backface-visibility: hidden;
  border-radius: 20px;
  padding: 30px;
  display: flex; flex-direction: column;
}
.card-back {
  position: absolute; top: 0; left: 0;
  transform: rotateY(180deg);
}

/* --- FORMS --- */
.input-group { margin-bottom: 15px; }
.input-field {
  width: 100%;
  background: rgba(0, 0, 0, 0.1);
  border: 1px solid var(--glass-border);
  padding: 14px;
  border-radius: 10px;
  color: var(--text-primary);
  font-size: 0.95rem;
  outline: none;
  transition: border-color 0.3s;
}
.input-field:focus { border-color: var(--accent-color); }

.primary-btn {
  width: 100%; padding: 14px;
  border-radius: 10px;
  background: var(--text-primary); color: var(--bg-color);
  font-weight: 600; border: none;
  cursor: pointer; font-size: 0.95rem;
  margin-top: 10px;
  transition: transform 0.2s;
}
.primary-btn:hover { transform: scale(1.02); }

.social-container {
  display: flex; gap: 10px; margin-bottom: 20px;
}
.social-btn {
  flex: 1; padding: 10px;
  border-radius: 10px;
  background: transparent;
  border: 1px solid var(--glass-border);
  color: var(--text-primary);
  font-size: 0.85rem;
  cursor: pointer;
  display: flex; align-items: center; justify-content: center; gap: 8px;
  transition: all 0.2s;
}
.social-btn:hover { background: rgba(255,255,255,0.05); border-color: var(--glass-highlight); }

.divider {
  display: flex; align-items: center;
  margin: 20px 0; color: var(--text-secondary); font-size: 0.8rem;
}
.divider::before, .divider::after {
  content: ''; flex: 1; height: 1px; background: var(--glass-border);
}
.divider span { padding: 0 10px; }

.link-btn {
  background: none; border: none;
  color: var(--text-secondary);
  cursor: pointer; font-size: 0.85rem;
  margin-top: 15px; text-align: center; width: 100%;
}
.link-btn:hover { color: var(--text-primary); }

/* --- WIZARD --- */
.wizard-steps { display: flex; justify-content: space-between; margin-bottom: 20px; }
.step-dot { width: 10px; height: 10px; border-radius: 50%; background: var(--glass-border); }
.step-dot.active { background: var(--accent-color); box-shadow: 0 0 10px var(--accent-color); }

.plan-card {
  border: 1px solid var(--glass-border); padding: 15px; border-radius: 10px;
  margin-bottom: 10px; cursor: pointer; transition: all 0.2s;
}
.plan-card:hover, .plan-card.selected {
  border-color: var(--accent-color); background: rgba(59, 130, 246, 0.1);
}

/* --- DASHBOARD --- */
.dashboard-layout {
  display: flex; height: 100vh; width: 100vw;
}
.dashboard-content {
  flex: 1; padding: 40px; overflow-y: auto;
  margin-left: 280px;
}
.dashboard-card {
  padding: 24px; border-radius: 16px; margin-bottom: 24px;
}
table { width: 100%; border-collapse: collapse; margin-top: 10px; }
th { text-align: left; padding: 15px; color: var(--text-secondary); font-weight: 500; border-bottom: 1px solid var(--glass-border); }
td { padding: 15px; border-bottom: 1px solid rgba(255,255,255,0.03); }
`;

// --- COMPONENTS ---

const RegisterWizard = ({ onBack }: { onBack: () => void }) => {
  const [step, setStep] = useState(1);
  const [formData, setFormData] = useState({ company: '', email: '', password: '', plan: 'pro' });
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

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

const LoginScreen = () => {
  const [scrollProgress, setScrollProgress] = useState(0);
  const [isFlipped, setIsFlipped] = useState(false);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();
  const { login } = useAuth();

  // Scroll Logic
  useEffect(() => {
    const handleWheel = (e: WheelEvent) => {
      setScrollProgress(prev => {
        const delta = e.deltaY * 0.1;
        return Math.min(Math.max(prev + delta, 0), 100);
      });
    };
    window.addEventListener('wheel', handleWheel);
    return () => window.removeEventListener('wheel', handleWheel);
  }, []);

  const gateOffset = Math.min(scrollProgress * 2, 100);
  const contentOpacity = Math.max((scrollProgress - 30) / 20, 0);
  const contentScale = Math.min(0.8 + (Math.max(scrollProgress - 30, 0) / 100), 1);

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
    <>
      {/* Gates */}
      <div className="gate-container">
        <div className="gate gate-left" style={{ transform: `translateX(-${gateOffset}%)` }}></div>
        <div className="gate gate-right" style={{ transform: `translateX(${gateOffset}%)` }}></div>
        <div className="gate-content" style={{ opacity: 1 - (scrollProgress / 20), pointerEvents: 'none' }}>
          <div className="titan-logo">TITAN</div>
          <div className="scroll-hint">▼ Scroll to Enter</div>
        </div>
      </div>

      {/* Auth Card */}
      <div className="main-stage" style={{ opacity: Math.min(contentOpacity, 1), transform: `scale(${contentScale})` }}>
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
      </div>
    </>
  );
};

const Dashboard = () => {
  const { logout, user } = useAuth();
  const navigate = useNavigate();

  const handleLogout = () => {
    logout();
    navigate('/');
  };

  return (
    <div className="dashboard-layout">
      <div className="sidebar open" style={{ width: '260px', transform: 'none' }}>
        <div className="sidebar-header">TITAN</div>
        <nav style={{ flex: 1 }}>
          <div className="nav-item"><Home size={20} /> Home</div>
          <div className="nav-item active"><LayoutDashboard size={20} /> Dashboard</div>
          <div className="nav-item"><Users size={20} /> Clients</div>
          <div className="nav-item"><Settings size={20} /> Settings</div>
        </nav>
        <div className="nav-item" onClick={handleLogout} style={{ marginTop: 'auto', color: '#ef4444' }}>
          <LogOut size={20} /> Logout
        </div>
      </div>

      <div className="dashboard-content">
        <header style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '40px' }}>
          <div>
            <h1 style={{ fontSize: '2rem', fontWeight: 'bold' }}>Dashboard</h1>
            <p style={{ color: 'var(--text-secondary)' }}>Welcome back, {user?.email || 'User'}</p>
          </div>
        </header>

        <div className="glass dashboard-card">
          <h3 style={{ fontSize: '1.2rem', marginBottom: '20px' }}>System Status</h3>
          <p>All systems operational.</p>
        </div>
      </div>
    </div>
  );
};

const ForgotPassword = () => {
  const navigate = useNavigate();
  const [email, setEmail] = useState('');

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    alert('Password reset link sent to ' + email);
    navigate('/');
  };

  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh' }}>
      <div className="glass" style={{ padding: '40px', borderRadius: '20px', width: '400px' }}>
        <h2 style={{ marginBottom: '20px' }}>Reset Password</h2>
        <form onSubmit={handleSubmit}>
          <div className="input-group">
            <input type="email" placeholder="Enter your email" className="input-field" value={email} onChange={e => setEmail(e.target.value)} required />
          </div>
          <button type="submit" className="primary-btn">Send Reset Link</button>
        </form>
        <button onClick={() => navigate('/')} className="link-btn">Back to Login</button>
      </div>
    </div>
  );
};

// --- MAIN APP COMPONENT ---
const App = () => {
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  const [lang, setLang] = useState('en');
  const [theme, setTheme] = useState('dark');
  const location = useLocation();

  const toggleTheme = () => {
    const newTheme = theme === 'dark' ? 'light' : 'dark';
    setTheme(newTheme);
    if (newTheme === 'light') {
      document.body.classList.add('light-mode');
    } else {
      document.body.classList.remove('light-mode');
    }
  };

  const isAuthPage = location.pathname === '/' || location.pathname === '/forgot-password';

  return (
    <AuthProvider>
      <style>{cssStyles}</style>

      {/* GLOBAL CONTROLS (Only on Auth Page) */}
      {isAuthPage && (
        <>
          <div className="top-left-controls">
            <button className="glass-button icon-btn" onClick={() => setIsSidebarOpen(!isSidebarOpen)}>
              {isSidebarOpen ? <X size={24} /> : <Menu size={24} />}
            </button>
          </div>

          <div className="top-right-controls">
            <div className="lang-selector">
              <button className="glass-button lang-btn">
                {translations[lang].flag} <ChevronDown size={14} />
              </button>
              <div className="lang-dropdown">
                {Object.keys(translations).map((l) => (
                  <div key={l} className="lang-option" onClick={() => setLang(l)}>
                    {translations[l].flag}
                  </div>
                ))}
              </div>
            </div>
            <button className="glass-button icon-btn" onClick={toggleTheme}>
              {theme === 'dark' ? <Moon size={20} /> : <Sun size={20} />}
            </button>
          </div>

          {/* Sidebar Overlay */}
          <div className={`sidebar ${isSidebarOpen ? 'open' : ''}`}>
            <h2 className="sidebar-header">TITAN</h2>
            <nav>
              <div className="nav-item"><Home size={20} /> Home</div>
              <div className="nav-item"><LayoutDashboard size={20} /> Dashboard</div>
              <div className="nav-item"><Users size={20} /> Clients</div>
              <div className="nav-item"><Settings size={20} /> Settings</div>
            </nav>
            <div className="nav-item" style={{ marginTop: 'auto', color: '#ef4444' }}>
              <LogOut size={20} /> Logout
            </div>
          </div>
        </>
      )}

      <Routes>
        <Route path="/" element={<LoginScreen />} />
        <Route path="/forgot-password" element={<ForgotPassword />} />
        <Route path="/dashboard" element={<ProtectedRoute><Dashboard /></ProtectedRoute>} />
      </Routes>
    </AuthProvider>
  );
};

const ProtectedRoute = ({ children }: { children: JSX.Element }) => {
  const { signed, loading } = useAuth();
  if (loading) return <div>Loading...</div>;
  return signed ? children : <Navigate to="/" />;
};

export default App;
