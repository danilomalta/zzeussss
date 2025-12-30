import React, { useState } from 'react';

// --- ESTILOS CSS ---
const styles = `
  @import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');

  :root {
    --bg-app: #050505;
    --gate-color: #050505;
    --card-bg: rgba(20, 20, 22, 0.95);
    --card-border: rgba(255, 255, 255, 0.1);
    --text-main: #FFFFFF;
    --text-muted: #A1A1AA;
    --accent: #3b82f6; 
    --input-bg: rgba(255, 255, 255, 0.05);
    --shadow: 0 20px 60px -10px rgba(0,0,0,0.9), 0 0 0 1px rgba(255,255,255,0.1);
  }

  .light-mode {
    --bg-app: #F5F5F7;
    --gate-color: #F5F5F7;
    --card-bg: rgba(255, 255, 255, 0.9);
    --card-border: rgba(0, 0, 0, 0.1);
    --text-main: #111111;
    --text-muted: #666666;
    --accent: #0070F3;
    --input-bg: #F0F0F0;
    --shadow: 0 10px 40px -5px rgba(0,0,0,0.15);
  }

  body { margin: 0; font-family: 'Inter', sans-serif; background: var(--bg-app); overflow: hidden; transition: background 0.3s; }
  * { box-sizing: border-box; outline: none; }

  .scroll-container { height: 100vh; overflow-y: auto; overflow-x: hidden; }
  .scroll-spacer { height: 200vh; }
  .viewport { position: fixed; inset: 0; display: flex; justify-content: center; align-items: center; pointer-events: none; z-index: 10; }

  .gate { position: absolute; top: 0; height: 100%; width: 50.2%; background: var(--gate-color); z-index: 20; will-change: transform; transition: background 0.3s; }
  .gate-left { left: 0; }
  .gate-right { right: 0; }

  .hero-title { position: absolute; z-index: 30; width: 100%; text-align: center; pointer-events: none; color: var(--text-main); mix-blend-mode: exclusion; transition: opacity 0.3s; }

  .scene { width: 380px; height: 580px; perspective: 1000px; z-index: 15; pointer-events: auto; opacity: 0; transform: scale(0.9); transition: opacity 0.5s, transform 0.5s; }
  .scene.visible { opacity: 1; transform: scale(1); }

  .card-inner { position: relative; width: 100%; height: 100%; transition: transform 0.8s cubic-bezier(0.175, 0.885, 0.32, 1.275); transform-style: preserve-3d; }
  .card-inner.flipped { transform: rotateY(180deg); }

  .card-face { position: absolute; inset: 0; backface-visibility: hidden; background: var(--card-bg); border: 1px solid var(--card-border); border-radius: 20px; box-shadow: var(--shadow); padding: 30px; display: flex; flex-direction: column; align-items: center; justify-content: center; backdrop-filter: blur(10px); }
  .card-face.back { transform: rotateY(180deg); }

  /* Planos */
  .plans-container { display: flex; gap: 10px; width: 100%; margin-bottom: 15px; }
  .plan-card { flex: 1; padding: 12px 5px; background: var(--input-bg); border: 1px solid var(--card-border); border-radius: 10px; text-align: center; cursor: pointer; transition: all 0.2s; }
  .plan-card.active { border-color: var(--accent); background: rgba(59, 130, 246, 0.1); box-shadow: 0 0 15px rgba(59, 130, 246, 0.2); }
  .plan-title { font-size: 11px; font-weight: 700; color: var(--text-main); margin-bottom: 4px; }
  .plan-price { font-size: 13px; font-weight: 800; color: var(--accent); }

  /* UI */
  .logo { font-size: 26px; font-weight: 700; color: var(--text-main); margin-bottom: 5px; }
  .desc { font-size: 13px; color: var(--text-muted); margin-bottom: 25px; }
  .titan-input { width: 100%; height: 42px; background: var(--input-bg); border: 1px solid transparent; border-radius: 8px; color: var(--text-main); font-size: 13px; padding: 0 12px; margin-bottom: 10px; }
  .titan-btn { width: 100%; height: 44px; margin-top: 10px; background: var(--accent); color: white; border: none; border-radius: 8px; font-size: 13px; font-weight: 600; cursor: pointer; }
  .link { font-size: 12px; color: var(--text-muted); margin-top: 15px; cursor: pointer; }
  
  /* Menu */
  .menu-btn { position: fixed; top: 25px; left: 25px; z-index: 1000; width: 45px; height: 45px; border-radius: 50%; display: flex; align-items: center; justify-content: center; color: var(--text-main); cursor: pointer; background: rgba(0,0,0,0.2); backdrop-filter: blur(10px); border: 1px solid var(--card-border); transition: transform 0.3s; }
  .menu-btn.active { transform: translateX(260px); background: var(--bg-app); }
  .sidebar { position: fixed; top: 0; left: 0; height: 100%; width: 280px; background: rgba(5, 5, 5, 0.8); backdrop-filter: blur(20px); border-right: 1px solid var(--card-border); z-index: 999; transform: translateX(-100%); transition: transform 0.3s; padding: 30px; padding-top: 90px; }
  .sidebar.open { transform: translateX(0); }
  .nav-item { padding: 15px 0; border-bottom: 1px solid var(--card-border); color: var(--text-muted); cursor: pointer; font-size: 14px; }
  .theme-btn { position: fixed; bottom: 25px; right: 25px; z-index: 1000; width: 45px; height: 45px; border-radius: 50%; background: var(--card-bg); border: 1px solid var(--card-border); display: flex; align-items: center; justify-content: center; color: var(--text-main); cursor: pointer; }
`;

export default function App() {
  const [scroll, setScroll] = useState(0);
  const [isFlipped, setIsFlipped] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const [isDark, setIsDark] = useState(true);
  const [frontView, setFrontView] = useState('login');
  const [userType, setUserType] = useState('company');
  const [selectedPlan, setSelectedPlan] = useState('pro');

  const onScroll = (e: any) => {
    const total = e.target.scrollHeight - e.target.clientHeight;
    setScroll(Math.min(e.target.scrollTop / (total * 0.45), 1));
  };

  const gateX = scroll * 100;
  const showCard = scroll > 0.15;

  return (
    <div className={isDark ? '' : 'light-mode'}>
      <style>{styles}</style>
      <div className="scroll-container" onScroll={onScroll}>
        <div className="scroll-spacer"></div>
      </div>

      <div className={`menu-btn ${menuOpen ? 'active' : ''}`} onClick={() => setMenuOpen(!menuOpen)}>
        {menuOpen ? '✕' : '☰'}
      </div>

      <div className={`sidebar ${menuOpen ? 'open' : ''}`}>
        <h3 style={{ marginTop: 0, color: 'var(--text-main)' }}>Navegação</h3>
        <div className="nav-item">🏠 Dashboard</div>
        <div className="nav-item">📦 Serviços</div>
        <div className="nav-item">⚙ Configurações</div>
      </div>

      <div className="theme-btn" onClick={() => setIsDark(!isDark)}>{isDark ? '☀' : '🌙'}</div>

      <div className="viewport">
        <div className="hero-title" style={{ opacity: 1 - scroll * 4, transform: `scale(${1 + scroll * 0.2})` }}>
          <h1 style={{ fontSize: '4rem', margin: 0, fontWeight: 800 }}>TITAN</h1>
          <p style={{ fontSize: '11px', letterSpacing: '3px', marginTop: '15px' }}>▼ ROLE PARA ENTRAR</p>
        </div>

        <div className="gate gate-left" style={{ transform: `translateX(-${gateX}%)` }}></div>
        <div className="gate gate-right" style={{ transform: `translateX(${gateX}%)` }}></div>

        <div className={`scene ${showCard ? 'visible' : ''}`}>
          <div className={`card-inner ${isFlipped ? 'flipped' : ''}`}>
            
            {/* FRENTE */}
            <div className="card-face front">
              {frontView === 'login' && (
                <>
                  <div className="logo">Bem-vindo</div>
                  <div className="desc">Acesse sua conta Titan.</div>
                  <input className="titan-input" type="email" placeholder="Email" />
                  <input className="titan-input" type="password" placeholder="Senha" />
                  <button className="titan-btn">ENTRAR</button>
                  <div className="link" onClick={() => setFrontView('recovery')}>Esqueceu a senha?</div>
                  <div className="link" onClick={() => setIsFlipped(true)}>Não tem conta? <b>Cadastrar ↻</b></div>
                </>
              )}
              {frontView === 'recovery' && (
                <>
                  <div className="logo">Recuperar</div>
                  <div className="desc">Digite seu email.</div>
                  <input className="titan-input" type="email" placeholder="seu@email.com" />
                  <button className="titan-btn">ENVIAR LINK</button>
                  <div className="link" onClick={() => setFrontView('login')}>← Voltar</div>
                </>
              )}
            </div>

            {/* VERSO */}
            <div className="card-face back">
              <div className="logo" style={{fontSize: '22px'}}>Criar Conta</div>
              
              <div style={{ display: 'flex', gap: '10px', width: '100%', marginBottom: '20px' }}>
                 <button onClick={() => setUserType('company')} style={{ flex: 1, padding: '8px', background: userType === 'company' ? 'var(--accent)' : 'transparent', border: '1px solid var(--card-border)', borderRadius: '6px', color: userType === 'company' ? 'white' : 'var(--text-muted)', fontSize: '12px', cursor: 'pointer' }}>Empresa</button>
                 <button onClick={() => setUserType('staff')} style={{ flex: 1, padding: '8px', background: userType === 'staff' ? 'var(--accent)' : 'transparent', border: '1px solid var(--card-border)', borderRadius: '6px', color: userType === 'staff' ? 'white' : 'var(--text-muted)', fontSize: '12px', cursor: 'pointer' }}>Staff</button>
              </div>

              {userType === 'company' ? (
                <>
                  <div className="plans-container">
                    <div className={`plan-card ${selectedPlan === 'start' ? 'active' : ''}`} onClick={() => setSelectedPlan('start')}>
                      <div className="plan-title">START</div><div className="plan-price">R$99</div>
                    </div>
                    <div className={`plan-card ${selectedPlan === 'pro' ? 'active' : ''}`} onClick={() => setSelectedPlan('pro')}>
                      <div className="plan-title">PRO</div><div className="plan-price">R$199</div>
                    </div>
                    <div className={`plan-card ${selectedPlan === 'corp' ? 'active' : ''}`} onClick={() => setSelectedPlan('corp')}>
                      <div className="plan-title">CORP</div><div className="plan-price">$$$</div>
                    </div>
                  </div>
                  <input className="titan-input" placeholder="Nome da Empresa" />
                  <input className="titan-input" placeholder="CNPJ" />
                </>
              ) : (
                <div style={{marginBottom: '40px', width: '100%'}}>
                   <input className="titan-input" placeholder="Código de Convite" />
                </div>
              )}

              <button className="titan-btn" style={{ background: '#10B981', marginTop: 'auto' }}>CONFIRMAR</button>
              <div className="link" onClick={() => setIsFlipped(false)} style={{marginBottom: '0'}}><b>↻ Voltar</b></div>
            </div>

          </div>
        </div>
      </div>
    </div>
  );
}