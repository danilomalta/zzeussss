import React, { useState, UIEvent } from 'react';

// --- ESTILOS CSS (TITAN PRIME - CORRIGIDO) ---
const styles = `
  @import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap');

  :root {
    /* PALETA DE CORES (Modo Escuro) */
    --bg-app: #020202; /* Preto mais profundo */
    --gate-color: #020202;
    --text-primary: #FFFFFF;
    --text-secondary: #A1A1AA;
    --accent-color: #3B82F6; /* Azul Tech */
    
    /* VIDRO E SOMBRAS (Corrigidas) */
    --glass-bg: rgba(30, 30, 35, 0.85);
    --glass-border: rgba(255, 255, 255, 0.08);
    --glass-blur: blur(25px);
    /* Sombra suave e moderna, não gigante */
    --card-shadow: 0 8px 32px 0 rgba(0, 0, 0, 0.3);
  }

  .light-mode {
    --bg-app: #F3F4F6;
    --gate-color: #F3F4F6;
    --text-primary: #111827;
    --text-secondary: #4B5563;
    --glass-bg: rgba(255, 255, 255, 0.85);
    --glass-border: rgba(0, 0, 0, 0.05);
    --card-shadow: 0 8px 32px 0 rgba(0, 0, 0, 0.1);
  }

  body { margin: 0; background: var(--bg-app); font-family: 'Inter', sans-serif; overflow: hidden; color: var(--text-primary); transition: background 0.3s ease; }
  * { box-sizing: border-box; outline: none; }

  /* CONTAINER DE ROLAGEM */
  .scroll-container { height: 100vh; overflow-y: auto; overflow-x: hidden; scroll-behavior: smooth; }
  .scroll-spacer { height: 180vh; } /* Altura ajustada para abrir mais rápido */
  .viewport { position: fixed; inset: 0; display: flex; justify-content: center; align-items: center; pointer-events: none; z-index: 10; }

  /* PORTÕES (GATES) - CORREÇÃO DA LINHA NO MEIO */
  .gate { 
    position: absolute; top: 0; height: 100%; 
    width: calc(50% + 2px); /* O SEGREDO: Sobreposição de 2px para matar a linha */
    background: var(--gate-color); z-index: 30; /* Z-index menor que os botões */
    will-change: transform; transition: transform 0.1s linear;
  }
  .gate-left { left: 0; transform-origin: left; border-right: 1px solid var(--glass-border); }
  .gate-right { right: 0; transform-origin: right; border-left: 1px solid var(--glass-border); }

  /* TEXTO INICIAL (Hero) */
  .hero-brand {
    position: absolute; z-index: 40; width: 100%; text-align: center; pointer-events: none;
    transition: opacity 0.3s, transform 0.3s; mix-blend-mode: exclusion;
  }
  .hero-brand h1 { font-size: 7rem; margin: 0; font-weight: 900; letter-spacing: -5px; }
  .scroll-indicator { font-size: 12px; letter-spacing: 4px; color: var(--text-secondary); margin-top: 20px; text-transform: uppercase; animation: pulse 2s infinite; }
  @keyframes pulse { 0% { opacity: 0.5; } 50% { opacity: 1; } 100% { opacity: 0.5; } }

  /* BOTÕES FLUTUANTES (Menu e Tema) - CORREÇÃO DE Z-INDEX */
  .float-btn {
    position: fixed; z-index: 9999; /* GARANTE QUE FIQUE NO TOPO */
    width: 48px; height: 48px; border-radius: 50%;
    background: var(--glass-bg); backdrop-filter: var(--glass-blur);
    border: 1px solid var(--glass-border);
    display: flex; align-items: center; justify-content: center;
    cursor: pointer; transition: all 0.3s ease;
    color: var(--text-primary); pointer-events: auto;
    box-shadow: 0 4px 15px rgba(0,0,0,0.2);
  }
  .float-btn:hover { border-color: var(--accent-color); transform: scale(1.05); }
  .menu-pos { top: 25px; left: 25px; }
  .theme-pos { bottom: 25px; right: 25px; }

  /* SIDEBAR (Menu Lateral) */
  .sidebar {
    position: fixed; top: 0; left: 0; height: 100%; width: 300px;
    background: var(--glass-bg); backdrop-filter: var(--glass-blur);
    border-right: 1px solid var(--glass-border);
    z-index: 9990; transform: translateX(-105%); transition: transform 0.4s cubic-bezier(0.16, 1, 0.3, 1);
    padding: 100px 30px 30px; pointer-events: auto;
  }
  .sidebar.open { transform: translateX(0); }
  .nav-item { padding: 15px 0; border-bottom: 1px solid var(--glass-border); color: var(--text-secondary); cursor: pointer; font-size: 15px; font-weight: 500; transition: 0.2s; }
  .nav-item:hover { color: var(--text-primary); padding-left: 10px; }

  /* CARTÃO 3D CENTRAL */
  .scene { width: 400px; height: 620px; perspective: 1500px; z-index: 25; pointer-events: auto; opacity: 0; transform: scale(0.9); transition: opacity 0.6s, transform 0.6s; }
  .scene.visible { opacity: 1; transform: scale(1); }
  .card-inner { position: relative; width: 100%; height: 100%; transition: transform 0.8s cubic-bezier(0.2, 0.8, 0.2, 1); transform-style: preserve-3d; }
  .card-inner.flipped { transform: rotateY(180deg); }

  /* ESTILO DAS FACES DO CARTÃO */
  .card-face {
    position: absolute; inset: 0; backface-visibility: hidden;
    background: var(--glass-bg); backdrop-filter: var(--glass-blur);
    border: 1px solid var(--glass-border); border-radius: 24px;
    padding: 40px; display: flex; flex-direction: column; align-items: center; justify-content: center;
    box-shadow: var(--card-shadow); /* Sombra Suave Aplicada */
  }
  .card-face.back { transform: rotateY(180deg); }

  /* INPUTS E BOTÕES */
  .titan-input {
    width: 100%; height: 52px; background: rgba(255,255,255,0.03);
    border: 1px solid var(--glass-border); border-radius: 12px;
    color: var(--text-primary); padding: 0 16px; font-size: 15px; margin-bottom: 12px; transition: 0.3s;
  }
  .titan-input:focus { border-color: var(--accent-color); background: rgba(59, 130, 246, 0.05); }
  .titan-btn {
    width: 100%; height: 52px; background: var(--accent-color); border: none; border-radius: 12px; color: white; font-weight: 700; cursor: pointer; transition: 0.3s; margin-top: 15px;
    box-shadow: 0 4px 20px rgba(59, 130, 246, 0.3);
  }
  .titan-btn:hover { transform: translateY(-2px); filter: brightness(1.1); }

  /* PLANOS (Cards Pequenos) */
  .plans-grid { display: flex; gap: 10px; width: 100%; margin: 20px 0; }
  .plan-card {
    flex: 1; padding: 15px 5px; background: rgba(255,255,255,0.02);
    border: 1px solid var(--glass-border); border-radius: 12px;
    text-align: center; cursor: pointer; transition: 0.3s; position: relative;
  }
  .plan-card.selected { border-color: var(--accent-color); background: rgba(59, 130, 246, 0.15); }
  .plan-name { font-size: 11px; font-weight: 800; color: var(--text-secondary); text-transform: uppercase; letter-spacing: 1px; }
  .plan-price { font-size: 18px; font-weight: 800; color: var(--text-primary); margin-top: 5px; }
`;

export default function App() {
  const [scroll, setScroll] = useState(0);
  const [isFlipped, setIsFlipped] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const [isDark, setIsDark] = useState(true);
  const [selectedPlan, setSelectedPlan] = useState('pro');
  const [userType, setUserType] = useState('company');

  const handleScroll = (e: UIEvent<HTMLDivElement>) => {
    const total = e.currentTarget.scrollHeight - e.currentTarget.clientHeight;
    setScroll(Math.min(e.currentTarget.scrollTop / (total * 0.5), 1));
  };
  const gateX = scroll * 100;
  const showCard = scroll > 0.1; // Abre mais rápido

  return (
    <div className={isDark ? '' : 'light-mode'}>
      <style>{styles}</style>
      
      {/* BOTÕES FLUTUANTES (Agora com Z-Index 9999) */}
      <button className="float-btn menu-pos" onClick={() => setMenuOpen(!menuOpen)}>
        {menuOpen ? '✕' : '☰'}
      </button>
      <button className="float-btn theme-pos" onClick={() => setIsDark(!isDark)}>
        {isDark ? '☀' : '🌙'}
      </button>

      {/* SIDEBAR */}
      <div className={`sidebar ${menuOpen ? 'open' : ''}`}>
        <h2 style={{fontSize: '24px', fontWeight: 800, marginBottom: '30px'}}>TITAN</h2>
        <div className="nav-item">🏠 Início</div>
        <div className="nav-item">🚀 Planos</div>
        <div className="nav-item">⚙ Configurações</div>
      </div>

      <div className="scroll-container" onScroll={handleScroll}>
        <div className="scroll-spacer"></div>
      </div>

      <div className="viewport">
        {/* TEXTO INICIAL */}
        <div className="hero-brand" style={{ opacity: 1 - scroll * 3, transform: `scale(${1 + scroll * 0.2})` }}>
          <h1>TITAN</h1>
          <div className="scroll-indicator">▼ Role para acessar ▼</div>
        </div>

        {/* PORTÕES (Sem linha no meio) */}
        <div className="gate gate-left" style={{ transform: `translateX(-${gateX}%)` }}></div>
        <div className="gate gate-right" style={{ transform: `translateX(${gateX}%)` }}></div>

        {/* CARTÃO CENTRAL */}
        <div className={`scene ${showCard ? 'visible' : ''}`}>
          <div className={`card-inner ${isFlipped ? 'flipped' : ''}`}>
            
            {/* --- FRENTE (Login) --- */}
            <div className="card-face front">
              <h2 style={{ fontSize: '28px', fontWeight: 800, marginBottom: '10px' }}>Login</h2>
              <p style={{ color: 'var(--text-secondary)', marginBottom: '30px' }}>Bem-vindo de volta.</p>
              <input className="titan-input" placeholder="Email Corporativo" />
              <input className="titan-input" type="password" placeholder="Senha" />
              <button className="titan-btn">ACESSAR SISTEMA</button>
              <div style={{marginTop: '20px', fontSize: '13px', color: 'var(--text-secondary)', cursor: 'pointer'}} onClick={() => setIsFlipped(true)}>
                Não tem conta? <b style={{color: 'var(--accent-color)'}}>Criar Agora ↻</b>
              </div>
            </div>

            {/* --- VERSO (Cadastro + Planos) --- */}
            <div className="card-face back">
              <h2 style={{ fontSize: '24px', fontWeight: 800, marginBottom: '20px' }}>Nova Conta</h2>
              
              {/* Seletor Tipo */}
              <div style={{display: 'flex', gap:'10px', width:'100%', marginBottom:'20px'}}>
                <button onClick={()=>setUserType('company')} style={{flex:1, padding:'10px', borderRadius:'10px', border:'1px solid var(--glass-border)', background: userType==='company'?'var(--accent-color)':'transparent', color:'white', fontWeight:700, cursor:'pointer'}}>Empresa</button>
                <button onClick={()=>setUserType('staff')} style={{flex:1, padding:'10px', borderRadius:'10px', border:'1px solid var(--glass-border)', background: userType==='staff'?'var(--accent-color)':'transparent', color: userType==='staff'?'white':'var(--text-secondary)', fontWeight:700, cursor:'pointer'}}>Equipe</button>
              </div>

              {userType === 'company' ? (
                <div style={{width:'100%'}}>
                   {/* TABELA DE PLANOS */}
                   <div className="plans-grid">
                      <div className={`plan-card ${selectedPlan === 'start' ? 'selected' : ''}`} onClick={() => setSelectedPlan('start')}>
                        <div className="plan-name">Start</div><div className="plan-price">R$99</div>
                      </div>
                      <div className={`plan-card ${selectedPlan === 'pro' ? 'selected' : ''}`} onClick={() => setSelectedPlan('pro')}>
                        <div className="plan-name">Pro</div><div className="plan-price">R$199</div>
                      </div>
                      <div className={`plan-card ${selectedPlan === 'corp' ? 'selected' : ''}`} onClick={() => setSelectedPlan('corp')}>
                        <div className="plan-name">Corp</div><div className="plan-price">$$$</div>
                      </div>
                   </div>
                   <input className="titan-input" placeholder="Nome da Empresa" />
                </div>
              ) : (
                 <input className="titan-input" placeholder="Código de Convite" style={{marginBottom:'40px'}} />
              )}

              <button className="titan-btn" style={{background:'#10B981', marginTop:'auto'}}>CONFIRMAR</button>
              <div style={{marginTop:'15px', fontSize:'13px', color: 'var(--text-secondary)', cursor: 'pointer'}} onClick={() => setIsFlipped(false)}>
                <b>↻ Voltar para Login</b>
              </div>
            </div>

          </div>
        </div>

      </div>
    </div>
  );
}