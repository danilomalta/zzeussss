import React, { useEffect, useState } from 'react';
import api from '../../services/api';
import Sidebar from '../../components/Layout/Sidebar';
import { ChevronLeft, ChevronRight } from 'lucide-react';

interface Client {
    ID: number;
    name: string;
    email: string;
    phone: string;
}

const ClientList: React.FC = () => {
    const [clients, setClients] = useState<Client[]>([]);
    const [page, setPage] = useState(1);
    const [totalPages, setTotalPages] = useState(1);
    const [loading, setLoading] = useState(false);
    const limit = 10; // Low limit for low RAM demo

    const fetchClients = async (pageNum: number) => {
        setLoading(true);
        try {
            const response = await api.get(`/clients?page=${pageNum}&limit=${limit}`);
            setClients(response.data.data);
            // Calculate total pages based on total count
            const total = response.data.total;
            setTotalPages(Math.ceil(total / limit));
        } catch (error) {
            console.error("Failed to fetch clients", error);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchClients(page);
    }, [page]);

    return (
        <div className="dashboard-layout">
            <Sidebar isOpen={true} />
            <div className="dashboard-content">
                <header style={{ marginBottom: '40px' }}>
                    <h1 style={{ fontSize: '2rem', fontWeight: 'bold' }}>Clients</h1>
                    <p style={{ color: 'var(--text-secondary)' }}>Manage your client base efficiently.</p>
                </header>

                <div className="glass dashboard-card">
                    {loading ? (
                        <p>Loading...</p>
                    ) : (
                        <>
                            <table>
                                <thead>
                                    <tr>
                                        <th>Name</th>
                                        <th>Email</th>
                                        <th>Phone</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {clients.map(client => (
                                        <tr key={client.ID}>
                                            <td>{client.name}</td>
                                            <td>{client.email}</td>
                                            <td>{client.phone}</td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>

                            {/* Pagination Controls */}
                            <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', gap: '20px', marginTop: '20px' }}>
                                <button
                                    className="glass-button"
                                    disabled={page === 1}
                                    onClick={() => setPage(p => Math.max(1, p - 1))}
                                    style={{ padding: '10px', borderRadius: '50%', opacity: page === 1 ? 0.5 : 1 }}
                                >
                                    <ChevronLeft size={20} />
                                </button>
                                <span>Page {page} of {totalPages || 1}</span>
                                <button
                                    className="glass-button"
                                    disabled={page >= totalPages}
                                    onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                                    style={{ padding: '10px', borderRadius: '50%', opacity: page >= totalPages ? 0.5 : 1 }}
                                >
                                    <ChevronRight size={20} />
                                </button>
                            </div>
                        </>
                    )}
                </div>
            </div>
        </div>
    );
};

export default ClientList;
