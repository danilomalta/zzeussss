import React, { useState } from 'react';
import { X } from 'lucide-react';
import api from '../../services/api';

interface ForgotPasswordModalProps {
    isOpen: boolean;
    onClose: () => void;
}

const ForgotPasswordModal: React.FC<ForgotPasswordModalProps> = ({ isOpen, onClose }) => {
    const [email, setEmail] = useState('');
    const [loading, setLoading] = useState(false);
    const [success, setSuccess] = useState(false);
    const [error, setError] = useState('');

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setLoading(true);
        setError('');
        
        try {
            await api.post('/auth/forgot-password', { email });
            setSuccess(true);
            setTimeout(() => {
                onClose();
                setSuccess(false);
                setEmail('');
            }, 3000);
        } catch (err: any) {
            setError(err.response?.data?.error || 'Erro ao enviar email de recuperação');
        } finally {
            setLoading(false);
        }
    };

    if (!isOpen) return null;

    return (
        <div 
            className="modal-backdrop"
            onClick={onClose}
            style={{
                position: 'fixed',
                top: 0,
                left: 0,
                width: '100%',
                height: '100%',
                background: 'rgba(0, 0, 0, 0.7)',
                backdropFilter: 'blur(5px)',
                zIndex: 10000,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
            }}
        >
            <div 
                className="glass-panel"
                onClick={(e) => e.stopPropagation()}
                style={{
                    width: '100%',
                    maxWidth: '420px',
                    padding: '32px',
                    position: 'relative',
                }}
            >
                <button
                    onClick={onClose}
                    className="icon-btn"
                    style={{
                        position: 'absolute',
                        top: '16px',
                        right: '16px',
                        width: '32px',
                        height: '32px',
                    }}
                    aria-label="Fechar"
                >
                    <X size={18} />
                </button>

                {success ? (
                    <div style={{ textAlign: 'center', padding: '20px 0' }}>
                        <div style={{ fontSize: '3rem', marginBottom: '20px' }}>✅</div>
                        <h3 style={{ 
                            color: 'var(--accent-color)', 
                            marginBottom: '10px',
                            fontSize: '1.3rem'
                        }}>
                            Email Enviado!
                        </h3>
                        <p style={{ color: 'var(--text-secondary)' }}>
                            Verifique sua caixa de entrada para redefinir sua senha.
                        </p>
                    </div>
                ) : (
                    <>
                        <h2 className="holographic-text" style={{ 
                            fontSize: '1.5rem', 
                            marginBottom: '20px',
                            textAlign: 'center'
                        }}>
                            Recuperar Senha
                        </h2>
                        <p style={{ 
                            color: 'var(--text-secondary)', 
                            marginBottom: '24px',
                            fontSize: '0.9rem',
                            textAlign: 'center'
                        }}>
                            Digite seu email e enviaremos um link para redefinir sua senha.
                        </p>

                        <form onSubmit={handleSubmit}>
                            <div className="input-group">
                                <input
                                    type="email"
                                    placeholder="Seu email"
                                    className="input-field"
                                    value={email}
                                    onChange={(e) => setEmail(e.target.value)}
                                    required
                                    autoFocus
                                />
                            </div>

                            {error && (
                                <div style={{
                                    padding: '12px',
                                    background: 'rgba(255, 59, 48, 0.1)',
                                    border: '1px solid var(--danger-color)',
                                    borderRadius: '8px',
                                    color: 'var(--danger-color)',
                                    marginBottom: '20px',
                                    fontSize: '0.9rem'
                                }}>
                                    {error}
                                </div>
                            )}

                            <button
                                type="submit"
                                className="primary-btn"
                                disabled={loading}
                            >
                                {loading ? 'Enviando...' : 'Enviar Link de Recuperação'}
                            </button>
                        </form>
                    </>
                )}
            </div>
        </div>
    );
};

export default ForgotPasswordModal;

