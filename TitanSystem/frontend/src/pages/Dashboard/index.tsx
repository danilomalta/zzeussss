import React from 'react';
import { useAuth } from '../../context/AuthContext';

const Dashboard: React.FC = () => {
    const { user } = useAuth();

    return (
        <div>
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
    );
};

export default Dashboard;
