

const WebLite = () => {
    return (
        <div className="dashboard-layout">
            <div style={{ padding: '40px', width: '100%' }}>
                <h1 className="holographic-text" style={{ fontSize: '2.5rem', marginBottom: '40px' }}>WEB PANEL</h1>

                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))', gap: '20px' }}>
                    {/* Subscription Card */}
                    <div className="glass-panel" style={{ padding: '30px' }}>
                        <h3 style={{ color: 'var(--accent-color)', marginBottom: '20px' }}>Minha Assinatura</h3>
                        <div style={{ fontSize: '2rem', fontWeight: 'bold', marginBottom: '10px' }}>TITAN PRO</div>
                        <p style={{ color: 'var(--text-secondary)' }}>Renova em: 15/02/2026</p>
                        <button className="titan-btn" style={{ marginTop: '20px', width: '100%' }}>Gerenciar</button>
                    </div>

                    {/* Invoices Card */}
                    <div className="glass-panel" style={{ padding: '30px' }}>
                        <h3 style={{ color: 'var(--accent-color)', marginBottom: '20px' }}>Faturas Recentes</h3>
                        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '15px', borderBottom: '1px solid var(--glass-border)', paddingBottom: '10px' }}>
                            <span>Jan 2026</span>
                            <span style={{ color: 'var(--success-color)' }}>Paga</span>
                        </div>
                        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '15px', borderBottom: '1px solid var(--glass-border)', paddingBottom: '10px' }}>
                            <span>Dez 2025</span>
                            <span style={{ color: 'var(--success-color)' }}>Paga</span>
                        </div>
                        <button className="titan-btn" style={{ marginTop: '10px', width: '100%', background: 'transparent', border: '1px solid var(--glass-border)', color: 'white' }}>Ver Todas</button>
                    </div>

                    {/* Download App Card */}
                    <div className="glass-panel" style={{ padding: '30px', border: '1px solid var(--accent-color)' }}>
                        <h3 style={{ color: 'var(--accent-color)', marginBottom: '20px' }}>Baixe o App Desktop</h3>
                        <p style={{ color: 'var(--text-secondary)', marginBottom: '20px' }}>
                            Para acessar o ERP, PDV e CRM completo, você precisa do aplicativo Desktop Titan System.
                        </p>
                        <button className="titan-btn" style={{ width: '100%' }}>DOWNLOAD WINDOWS</button>
                        <button className="titan-btn" style={{ width: '100%', marginTop: '10px', background: 'transparent', border: '1px solid var(--glass-border)' }}>DOWNLOAD MAC</button>
                    </div>
                </div>
            </div>
        </div>
    );
};

export default WebLite;
