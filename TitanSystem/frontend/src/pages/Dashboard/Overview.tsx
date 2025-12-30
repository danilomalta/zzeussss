import React from 'react';
import { useAuth } from '../../context/AuthContext';
import Sidebar from '../../components/Layout/Sidebar';

const Overview: React.FC = () => {
    const { user } = useAuth();

    return (
        <div className="dashboard-layout">
            {/* Sidebar is always open on Dashboard */}
            <Sidebar isOpen={true} />

            <div className="dashboard-content">
                <header style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '40px' }}>
                    <div>
                        <h1 style={{ fontSize: '2rem', fontWeight: 'bold' }}>Dashboard</h1>
                        <p style={{ color: 'var(--text-secondary)' }}>Welcome back, {user?.email || 'User'}</p>
                    </div>
                </header>

                <div className="glass dashboard-card">
                    <h3 style={{ fontSize: '1.2rem', marginBottom: '20px' }}>System Status</h3>
                    <p>All systems operational.</p>
                </div>
            </div>
        </div>
    );
};

export default Overview;
