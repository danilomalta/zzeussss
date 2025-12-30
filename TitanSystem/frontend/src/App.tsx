import { useState } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { useAuth, AuthProvider } from './context/AuthContext';
import AuthLayout from './layouts/AuthLayout';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import ClientList from './pages/Clients/ClientList';

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

const ForgotPassword = () => {
  const [email, setEmail] = useState('');
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh' }}>
      <div className="glass" style={{ padding: '40px', borderRadius: '20px', width: '400px' }}>
        <h2 style={{ marginBottom: '20px' }}>Reset Password</h2>
        <div className="input-group">
          <input type="email" placeholder="Enter your email" className="input-field" value={email} onChange={e => setEmail(e.target.value)} required />
        </div>
        <button className="primary-btn">Send Reset Link</button>
      </div>
    </div>
  );
};

const App = () => {
  return (
    <AuthProvider>
      <style>{cssStyles}</style>
      <Routes>
        <Route element={<AuthLayout />}>
          <Route path="/" element={<Login />} />
          <Route path="/forgot-password" element={<ForgotPassword />} />
        </Route>
        <Route path="/dashboard" element={<ProtectedRoute><Dashboard /></ProtectedRoute>} />
        <Route path="/clients" element={<ProtectedRoute><ClientList /></ProtectedRoute>} />
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
