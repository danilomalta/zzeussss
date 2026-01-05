import React from 'react';
import { useNavigate, useLocation } from 'react-router-dom';

interface SidebarProps {
    isOpen: boolean;
    onClose: () => void;
}

const Sidebar: React.FC<SidebarProps> = ({ isOpen, onClose }) => {
    const navigate = useNavigate();
    const location = useLocation();

    const menuItems = [
        { label: 'Dashboard', path: '/dashboard', icon: '📊' },
        { label: 'PDV System', path: '/pdv', icon: '🛒' },
        { label: 'CRM Clients', path: '/crm', icon: '👥' },
        { label: 'Financial', path: '/financial', icon: '💰' },
        { label: 'Settings', path: '/settings', icon: '⚙️' },
    ];

    const handleNavigate = (path: string) => {
        navigate(path);
        onClose();
    };

    return (
        <>
            {/* Backdrop */}
            {isOpen && (
                <div
                    style={{
                        position: 'fixed',
                        top: 0,
                        left: 0,
                        width: '100%',
                        height: '100%',
                        background: 'rgba(0,0,0,0.7)',
                        zIndex: 9998,
                        backdropFilter: 'blur(5px)',
                    }}
                    onClick={onClose}
                />
            )}

            <div className={`sidebar ${isOpen ? 'open' : ''}`}>
                <div className="sidebar-header">
                    <h2>Titan Prime</h2>
                    <p>System v2.0</p>
                </div>

                <nav style={{ flex: 1 }}>
                    {menuItems.map((item) => (
                        <div
                            key={item.path}
                            className={`nav-item ${location.pathname === item.path ? 'active' : ''}`}
                            onClick={() => handleNavigate(item.path)}
                        >
                            <span style={{ fontSize: '1.2rem' }}>{item.icon}</span>
                            <span>{item.label}</span>
                        </div>
                    ))}
                </nav>

                <div style={{ marginTop: 'auto' }}>
                    <div 
                        className="nav-item" 
                        onClick={() => {
                            navigate('/');
                            onClose();
                        }} 
                        style={{ color: 'var(--danger-color)' }}
                    >
                        <span>🚪</span>
                        <span>Logout</span>
                    </div>
                </div>
            </div>
        </>
    );
};

export default Sidebar;
