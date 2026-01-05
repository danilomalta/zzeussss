import React, { useState } from 'react';
import { Outlet } from 'react-router-dom';
import Sidebar from '../components/Layout/Sidebar';
import LanguageSwitcher from '../components/features/LanguageSwitcher';

const AuthLayout: React.FC = () => {
    const [isSidebarOpen, setIsSidebarOpen] = useState(false);

    return (
        <div style={{ 
            position: 'relative', 
            width: '100vw', 
            height: '100vh', 
            overflow: 'hidden',
            background: 'var(--bg-app)'
        }}>
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

            {/* Sidebar */}
            <Sidebar isOpen={isSidebarOpen} onClose={() => setIsSidebarOpen(false)} />

            {/* Language Switcher (Bottom Right) */}
            <LanguageSwitcher />

            {/* Main Content - Login Card será posicionado absolutamente pelo CSS */}
            <div style={{ 
                position: 'relative', 
                width: '100%', 
                height: '100%'
            }}>
                <Outlet />
            </div>
        </div>
    );
};

export default AuthLayout;
