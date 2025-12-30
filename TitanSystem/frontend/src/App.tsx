import React, { useState, useEffect, useRef } from 'react';
import { Menu, Moon, Sun, ArrowRight, Check, Building2, Users, Briefcase, X, ChevronRight } from 'lucide-react';

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
  --accent-glow: rgba(59, 130, 246, 0.5);
  --neon-blue: #00f3ff;
}

* {
  box-sizing: border-box;
  margin: 0;
  padding: 0;
}

body {
  background-color: var(--bg-color);
  color: var(--text-primary);
  font-family: 'Inter', sans-serif;
  overflow: hidden; /* Prevent default scroll */
  height: 100vh;
  width: 100vw;
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
  color: white;
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
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  z-index: 40;
  pointer-events: none; /* Allow scroll/wheel to pass through if needed, but we handle wheel on window */
  display: flex;
}

.gate {
  height: 100%;
  width: 51%; /* Overlap fix */
  background-color: #020202;
  position: absolute;
  top: 0;
  transition: transform 0.1s linear; /* Smooth scroll sync */
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 41;
}

.gate-left {
  left: 0;
  border-right: 1px solid #111;
}

.gate-right {
  right: 0;
  border-left: 1px solid #111;
}

.gate-content {
  position: absolute;
  top: 50%;
  left: 50%;
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
  background: linear-gradient(to bottom, #fff, #666);
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
.floating-btn {
  position: fixed;
  width: 50px;
  height: 50px;
  border-radius: 50%;
  z-index: 100; /* Above gates */
}

.menu-btn {
  top: 30px;
  left: 30px;
}

.theme-btn {
  bottom: 30px;
  right: 30px;
}

/* --- SIDEBAR --- */
.sidebar {
  position: fixed;
  top: 0;
  left: 0;
  height: 100%;
  width: 300px;
  z-index: 90;
  transform: translateX(-100%);
  transition: transform 0.5s cubic-bezier(0.4, 0, 0.2, 1);
  padding: 80px 30px;
}

.sidebar.open {
  transform: translateX(0);
}

.sidebar-link {
  display: block;
  padding: 15px 0;
  color: var(--text-secondary);
  text-decoration: none;
  font-size: 1.2rem;
  border-bottom: 1px solid rgba(255,255,255,0.05);
  transition: color 0.3s;
}

.sidebar-link:hover {
  color: white;
  padding-left: 10px;
}

/* --- MAIN CONTENT (3D CARD) --- */
.main-stage {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10;
  perspective: 1500px;
}

.card-wrapper {
  width: 420px;
  height: 600px;
  position: relative;
  transform-style: preserve-3d;
  transition: transform 0.8s cubic-bezier(0.175, 0.885, 0.32, 1.275);
}

.card-wrapper.flipped {
  transform: rotateY(180deg);
}

.card-face {
  position: absolute;
  width: 100%;
  height: 100%;
  backface-visibility: hidden;
  border-radius: 24px;
  padding: 40px;
  display: flex;
  flex-direction: column;
}

.card-front {
  /* Default glass style */
}

.card-back {
  transform: rotateY(180deg);
}

/* --- FORMS --- */
.input-group {
  margin-bottom: 20px;
}

.input-field {
  width: 100%;
  background: rgba(0, 0, 0, 0.3);
  border: 1px solid var(--glass-border);
  padding: 16px;
  border-radius: 12px;
  color: white;
  font-size: 1rem;
  outline: none;
  transition: border-color 0.3s;
}

.input-field:focus {
  border-color: var(--accent-color);
}

.primary-btn {
  width: 100%;
  padding: 16px;
  border-radius: 12px;
  background: white;
  color: black;
  font-weight: 600;
  border: none;
  cursor: pointer;
  font-size: 1rem;
  margin-top: 10px;
  transition: transform 0.2s;
}

.primary-btn:hover {
  transform: scale(1.02);
}

.link-btn {
  background: none;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 0.9rem;
  margin-top: 20px;
  text-align: center;
  width: 100%;
}

.link-btn:hover {
  color: white;
}

/* --- PRICING TOGGLE --- */
.toggle-container {
  display: flex;
  background: rgba(0,0,0,0.3);
  padding: 4px;
  border-radius: 12px;
  margin-bottom: 20px;
}

.toggle-btn {
  flex: 1;
  padding: 10px;
  text-align: center;
  border-radius: 8px;
  cursor: pointer;
  color: var(--text-secondary);
  transition: all 0.3s;
  font-size: 0.9rem;
}

.toggle-btn.active {
  background: rgba(255,255,255,0.1);
  color: white;
  font-weight: 600;
}

/* --- PRICING CARDS --- */
.pricing-grid {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 10px;
  margin-bottom: 20px;
}

.price-card {
  background: rgba(255,255,255,0.02);
  border: 1px solid var(--glass-border);
  border-radius: 12px;
  padding: 15px 10px;
  text-align: center;
  cursor: pointer;
  transition: all 0.3s;
}

.price-card:hover {
  background: rgba(255,255,255,0.05);
}

.price-card.selected {
  border-color: var(--neon-blue);
  background: rgba(0, 243, 255, 0.05);
  box-shadow: 0 0 15px rgba(0, 243, 255, 0.1);
}

.plan-name {
  font-size: 0.8rem;
  color: var(--text-secondary);
  margin-bottom: 5px;
}

.plan-price {
  font-size: 1.1rem;
  font-weight: bold;
  color: white;
}

.glow-text {
  text-shadow: 0 0 10px rgba(255,255,255,0.3);
}
`;

const App = () => {
    // --- STATE ---
    const [scrollProgress, setScrollProgress] = useState(0); // 0 to 100
    const [isSidebarOpen, setIsSidebarOpen] = useState(false);
    const [isFlipped, setIsFlipped] = useState(false);
    const [registerType, setRegisterType] = useState<'company' | 'staff'>('company');
    const [selectedPlan, setSelectedPlan] = useState<'start' | 'pro' | 'corp'>('pro');

    // --- SCROLL LOGIC ---
    useEffect(() => {
        const handleWheel = (e: WheelEvent) => {
            // Prevent default scroll behavior is handled by overflow: hidden on body in CSS
            // But we can also preventDefault here to be safe if we were using a scrollable container
            // e.preventDefault(); 

            setScrollProgress(prev => {
                const delta = e.deltaY * 0.1; // Sensitivity
                const newProgress = Math.min(Math.max(prev + delta, 0), 100);
                return newProgress;
            });
        };

        window.addEventListener('wheel', handleWheel);
        return () => window.removeEventListener('wheel', handleWheel);
    }, []);

    // --- DERIVED VALUES ---
    // Gates open fully at 50% progress
    const gateOffset = Math.min(scrollProgress * 2, 100);
    const contentOpacity = Math.max((scrollProgress - 30) / 20, 0);
    const contentScale = Math.min(0.8 + (Math.max(scrollProgress - 30, 0) / 100), 1);

    // --- RENDER ---
    return (
        <>
            <style>{cssStyles}</style>

            {/* --- FLOATING BUTTONS --- */}
            <button
                className="glass-button floating-btn menu-btn"
                onClick={() => setIsSidebarOpen(!isSidebarOpen)}
            >
                {isSidebarOpen ? <X size={24} /> : <Menu size={24} />}
            </button>

            <button className="glass-button floating-btn theme-btn">
                <Moon size={24} />
            </button>

            {/* --- SIDEBAR --- */}
            <div className={`glass sidebar ${isSidebarOpen ? 'open' : ''}`}>
                <h2 style={{ marginBottom: '40px', fontSize: '1.5rem', fontWeight: 'bold' }}>TITAN</h2>
                <nav>
                    <a href="#" className="sidebar-link">Dashboard</a>
                    <a href="#" className="sidebar-link">Services</a>
                    <a href="#" className="sidebar-link">Settings</a>
                    <a href="#" className="sidebar-link">Support</a>
                </nav>
            </div>

            {/* --- GATES --- */}
            <div className="gate-container">
                {/* Left Gate */}
                <div
                    className="gate gate-left"
                    style={{ transform: `translateX(-${gateOffset}%)` }}
                >
                    <div className="gate-content" style={{ opacity: 1 - (scrollProgress / 30) }}>
                        {/* Content attached to left gate if needed, or centered absolutely */}
                    </div>
                </div>

                {/* Right Gate */}
                <div
                    className="gate gate-right"
                    style={{ transform: `translateX(${gateOffset}%)` }}
                >
                </div>

                {/* Centered Text (Fades out) */}
                <div
                    className="gate-content"
                    style={{
                        opacity: 1 - (scrollProgress / 20),
                        pointerEvents: 'none'
                    }}
                >
                    <div className="titan-logo">TITAN</div>
                    <div className="scroll-hint">▼ Scroll to Enter</div>
                </div>
            </div>

            {/* --- MAIN STAGE (3D CARD) --- */}
            <div
                className="main-stage"
                style={{
                    opacity: Math.min(contentOpacity, 1),
                    transform: `scale(${contentScale})`
                }}
            >
                <div className={`card-wrapper ${isFlipped ? 'flipped' : ''}`}>

                    {/* FRONT: LOGIN */}
                    <div className="glass card-face card-front">
                        <div style={{ marginBottom: '30px', textAlign: 'center' }}>
                            <h2 className="glow-text" style={{ fontSize: '2rem', marginBottom: '10px' }}>Welcome</h2>
                            <p style={{ color: 'var(--text-secondary)' }}>Access the Titan System</p>
                        </div>

                        <div className="input-group">
                            <input type="email" placeholder="Email Address" className="input-field" />
                        </div>
                        <div className="input-group">
                            <input type="password" placeholder="Password" className="input-field" />
                        </div>

                        <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: '20px' }}>
                            <a href="#" style={{ color: 'var(--text-secondary)', fontSize: '0.8rem', textDecoration: 'none' }}>Forgot Password?</a>
                        </div>

                        <button className="primary-btn">Enter System</button>

                        <button onClick={() => setIsFlipped(true)} className="link-btn">
                            Create New Account <ArrowRight size={14} style={{ display: 'inline', marginLeft: '5px' }} />
                        </button>
                    </div>

                    {/* BACK: REGISTER */}
                    <div className="glass card-face card-back">
                        <h2 style={{ fontSize: '1.5rem', marginBottom: '20px', textAlign: 'center' }}>Join Titan</h2>

                        {/* Toggle */}
                        <div className="toggle-container">
                            <div
                                className={`toggle-btn ${registerType === 'company' ? 'active' : ''}`}
                                onClick={() => setRegisterType('company')}
                            >
                                Company
                            </div>
                            <div
                                className={`toggle-btn ${registerType === 'staff' ? 'active' : ''}`}
                                onClick={() => setRegisterType('staff')}
                            >
                                Staff
                            </div>
                        </div>

                        {registerType === 'company' ? (
                            <>
                                {/* Pricing Table */}
                                <div className="pricing-grid">
                                    <div
                                        className={`price-card ${selectedPlan === 'start' ? 'selected' : ''}`}
                                        onClick={() => setSelectedPlan('start')}
                                    >
                                        <div className="plan-name">Start</div>
                                        <div className="plan-price">$29</div>
                                    </div>
                                    <div
                                        className={`price-card ${selectedPlan === 'pro' ? 'selected' : ''}`}
                                        onClick={() => setSelectedPlan('pro')}
                                    >
                                        <div className="plan-name">Pro</div>
                                        <div className="plan-price">$99</div>
                                    </div>
                                    <div
                                        className={`price-card ${selectedPlan === 'corp' ? 'selected' : ''}`}
                                        onClick={() => setSelectedPlan('corp')}
                                    >
                                        <div className="plan-name">Corp</div>
                                        <div className="plan-price">$299</div>
                                    </div>
                                </div>

                                <div className="input-group">
                                    <input type="text" placeholder="Company Name" className="input-field" />
                                </div>
                                <div className="input-group">
                                    <input type="text" placeholder="CNPJ / Tax ID" className="input-field" />
                                </div>
                            </>
                        ) : (
                            <div className="input-group" style={{ marginTop: '40px', marginBottom: '40px' }}>
                                <label style={{ display: 'block', marginBottom: '10px', color: 'var(--text-secondary)' }}>Invitation Code</label>
                                <input type="text" placeholder="XYZ-123-TITAN" className="input-field" style={{ textAlign: 'center', letterSpacing: '2px', fontSize: '1.2rem' }} />
                            </div>
                        )}

                        <div style={{ marginTop: 'auto' }}>
                            <button className="primary-btn">Confirm Registration</button>
                            <button onClick={() => setIsFlipped(false)} className="link-btn">
                                Back to Login
                            </button>
                        </div>
                    </div>

                </div>
            </div>
        </>
    );
};

export default App;
