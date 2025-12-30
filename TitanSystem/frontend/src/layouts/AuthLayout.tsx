import React, { useState, useEffect } from 'react';
import { Outlet } from 'react-router-dom';
import { Menu, Moon, Sun, ChevronDown, X } from 'lucide-react';
import Sidebar from '../components/Layout/Sidebar';

// --- TRANSLATIONS ---
const translations: any = {
    en: { flag: "🇺🇸 EN" },
    pt: { flag: "🇧🇷 PT" },
    fr: { flag: "🇫🇷 FR" },
    cn: { flag: "🇨🇳 CN" },
    jp: { flag: "🇯🇵 JP" },
    ar: { flag: "🇸🇦 AR" }
};

const AuthLayout: React.FC = () => {
    const [isSidebarOpen, setIsSidebarOpen] = useState(false);
    const [lang, setLang] = useState('en');
    const [theme, setTheme] = useState('dark');
    const [scrollProgress, setScrollProgress] = useState(0);

    const toggleTheme = () => {
        const newTheme = theme === 'dark' ? 'light' : 'dark';
        setTheme(newTheme);
        if (newTheme === 'light') {
            document.body.classList.add('light-mode');
        } else {
            document.body.classList.remove('light-mode');
        }
    };

    // Scroll Logic for Gates
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

    return (
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

            <Sidebar isOpen={isSidebarOpen} />

            {/* Gates */}
            <div className="gate-container">
                <div className="gate gate-left" style={{ transform: `translateX(-${gateOffset}%)` }}></div>
                <div className="gate gate-right" style={{ transform: `translateX(${gateOffset}%)` }}></div>
                <div className="gate-content" style={{ opacity: 1 - (scrollProgress / 20), pointerEvents: 'none' }}>
                    <div className="titan-logo">TITAN</div>
                    <div className="scroll-hint">▼ Scroll to Enter</div>
                </div>
            </div>

            {/* Main Content Stage */}
            <div className="main-stage" style={{ opacity: Math.min(contentOpacity, 1), transform: `scale(${contentScale})` }}>
                <Outlet />
            </div>
        </>
    );
};

export default AuthLayout;
