import React from 'react';
import { useNavigate } from 'react-router-dom';
import { Home, LayoutDashboard, Users, Settings, LogOut } from 'lucide-react';
import { useAuth } from '../../context/AuthContext';

interface SidebarProps {
    isOpen: boolean;
}

const Sidebar: React.FC<SidebarProps> = ({ isOpen }) => {
    const { logout } = useAuth();
    const navigate = useNavigate();

    const handleLogout = () => {
        logout();
        navigate('/');
    };

    return (
        <div className={`glass sidebar ${isOpen ? 'open' : ''}`}>
            <h2 className="sidebar-header">TITAN</h2>
            <nav style={{ flex: 1 }}>
                <div className="nav-item"><Home size={20} /> Home</div>
                <div className="nav-item active" onClick={() => navigate('/dashboard')}><LayoutDashboard size={20} /> Dashboard</div>
                <div className="nav-item" onClick={() => navigate('/clients')}><Users size={20} /> Clients</div>
                <div className="nav-item"><Settings size={20} /> Settings</div>
            </nav>
            <div className="nav-item" onClick={handleLogout} style={{ marginTop: 'auto', color: '#ef4444' }}>
                <LogOut size={20} /> Logout
            </div>
        </div>
    );
};

export default Sidebar;
