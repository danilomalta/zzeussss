import React, { useState } from 'react';
import Sidebar from './Sidebar';
import { useTheme } from '../../context/ThemeContext';
import { useLanguage } from '../../context/LanguageContext';

interface MainLayoutProps {
    children: React.ReactNode;
}

const MainLayout: React.FC<MainLayoutProps> = ({ children }) => {
    const [isSidebarOpen, setIsSidebarOpen] = useState(false);
    const { theme, toggleTheme } = useTheme();
    const { lang, setLang } = useLanguage();

    const toggleLanguage = () => {
        setLang(lang === 'pt' ? 'en' : 'pt');
    };

    return (
        <div style={{ position: 'relative', minHeight: '100vh', width: '100vw', overflow: 'hidden' }}>
            {/* Top Left - Hamburger Menu (FIXO) */}
            <div className="top-left-controls">
                <button 
                    className="icon-btn" 
                    onClick={() => setIsSidebarOpen(true)}
                    aria-label="Abrir menu"
                >
                    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                        <line x1="3" y1="12" x2="21" y2="12"></line>
                        <line x1="3" y1="6" x2="21" y2="6"></line>
                        <line x1="3" y1="18" x2="21" y2="18"></line>
                    </svg>
                </button>
            </div>

            {/* Top Right - Controls (FIXO) */}
            <div className="top-right-controls">
                <button 
                    className="icon-btn" 
                    onClick={toggleLanguage}
                    aria-label={`Trocar idioma para ${lang === 'pt' ? 'Inglês' : 'Português'}`}
                    title={`Idioma: ${lang.toUpperCase()}`}
                >
                    {lang === 'pt' ? '🇧🇷' : '🇺🇸'}
                </button>
                <button 
                    className="icon-btn" 
                    onClick={toggleTheme}
                    aria-label={`Trocar tema para ${theme === 'dark' ? 'Claro' : 'Escuro'}`}
                    title={`Tema: ${theme === 'dark' ? 'Escuro' : 'Claro'}`}
                >
                    {theme === 'dark' ? '🌙' : '☀️'}
                </button>
            </div>

            {/* Sidebar */}
            <Sidebar isOpen={isSidebarOpen} onClose={() => setIsSidebarOpen(false)} />

            {/* Main Content */}
            <main style={{
                padding: '80px 40px 40px 40px',
                height: '100vh',
                overflowY: 'auto',
                transition: 'filter 0.3s',
                filter: isSidebarOpen ? 'blur(5px)' : 'none',
                pointerEvents: isSidebarOpen ? 'none' : 'auto',
            }}>
                {children}
            </main>
        </div>
    );
};

export default MainLayout;
