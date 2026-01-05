import { Routes, Route, Navigate, useLocation } from 'react-router-dom';
import { useState, useEffect } from 'react';
import { Globe } from 'lucide-react';
import Login from './pages/Login';
import RegisterWizard from './pages/Auth/RegisterWizard';
import WebLite from './pages/WebLite/WebLite';
import Dashboard from './pages/Dashboard';
import './index.css';

// --- ENVIRONMENT DETECTION ---
const isElectron = () => {
    const userAgent = navigator.userAgent.toLowerCase();
    return userAgent.indexOf(' electron/') > -1;
};

// --- GUARDS ---
const PlatformGuard = ({ children }: { children: JSX.Element }) => {
    const isApp = isElectron();
    if (!isApp) return <Navigate to="/web-lite" replace />;
    return children;
};

const WebGuard = ({ children }: { children: JSX.Element }) => {
    // Allow web users to see WebLite
    return children;
};

const App = () => {
    const [lang, setLang] = useState('EN');
    const location = useLocation();

    // Auto-detect theme (always dark for Titanium)
    useEffect(() => {
        document.body.classList.add('dark');
    }, []);

    return (
        <>
            <Routes>
                {/* PUBLIC */}
                <Route path="/" element={<Login />} />
                <Route path="/register" element={<RegisterWizard />} />

                {/* WEB LITE (Browser) */}
                <Route path="/web-lite" element={
                    <WebGuard>
                        <WebLite />
                    </WebGuard>
                } />

                {/* DASHBOARD (Electron Only) */}
                <Route path="/dashboard" element={
                    <PlatformGuard>
                        <Dashboard />
                    </PlatformGuard>
                } />

                {/* Fallback */}
                <Route path="*" element={<Navigate to="/" />} />
            </Routes>

            {/* LANGUAGE CONTROL (Fixed Bottom Right) */}
            {location.pathname === '/' && (
                <div className="lang-control">
                    <button className="lang-btn" onClick={() => setLang(lang === 'EN' ? 'PT' : 'EN')}>
                        <Globe size={16} /> {lang}
                    </button>
                </div>
            )}
        </>
    );
};

export default App;
