import { Routes, Route, Navigate } from 'react-router-dom';
import { useAuth, AuthProvider } from './context/AuthContext';
import { ThemeProvider } from './context/ThemeContext';
import { LanguageProvider } from './context/LanguageContext';
import AuthLayout from './layouts/AuthLayout';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import ClientList from './pages/Clients/ClientList';
import RegisterWizard from './pages/Auth/RegisterWizard';
import WebLite from './pages/WebLite/WebLite';
import MainLayout from './components/Layout/MainLayout';
import './index.css';

// --- ENVIRONMENT DETECTION ---
const isElectron = (): boolean => {
    const userAgent = navigator.userAgent.toLowerCase();
    return userAgent.indexOf(' electron/') > -1;
};

const isBrowser = (): boolean => {
    return !isElectron();
};

// --- GUARDS ---
const ProtectedRoute = ({ children }: { children: JSX.Element }) => {
    const { signed, loading } = useAuth();
    if (loading) {
        return (
            <div className="glass-panel" style={{ 
                padding: 40, 
                color: 'white',
                position: 'fixed',
                top: '50%',
                left: '50%',
                transform: 'translate(-50%, -50%)',
                zIndex: 9999
            }}>
                Carregando Titan System...
            </div>
        );
    }
    return signed ? children : <Navigate to="/" />;
};

// Guard para rotas apenas Desktop (Electron)
const DesktopOnlyRoute = ({ children }: { children: JSX.Element }) => {
    const isApp = isElectron();

    if (!isApp) {
        // Se for WEB, redireciona para /web-panel
        return <Navigate to="/web-panel" replace />;
    }

    // Se for APP (Electron), permite acesso
    return children;
};

// Guard para rotas apenas Web (Browser)
const WebOnlyRoute = ({ children }: { children: JSX.Element }) => {
    const isApp = isElectron();

    if (isApp) {
        // Se for Electron, redireciona para dashboard
        return <Navigate to="/dashboard" replace />;
    }

    // Se for Browser, permite acesso
    return children;
};

const App = () => {
    return (
        <ThemeProvider>
            <LanguageProvider>
                <AuthProvider>
                    <Routes>
                        {/* PUBLIC ROUTES */}
                        <Route element={<AuthLayout />}>
                            <Route path="/" element={<Login />} />
                            <Route path="/login" element={<Login />} />
                        </Route>

                        <Route path="/register" element={<RegisterWizard />} />

                        {/* WEB PANEL (Apenas Browser - Faturas, Status, Download) */}
                        <Route 
                            path="/web-panel" 
                            element={
                                <ProtectedRoute>
                                    <WebOnlyRoute>
                                        <MainLayout>
                                            <WebLite />
                                        </MainLayout>
                                    </WebOnlyRoute>
                                </ProtectedRoute>
                            } 
                        />

                        {/* Alias para compatibilidade */}
                        <Route 
                            path="/web-lite" 
                            element={<Navigate to="/web-panel" replace />} 
                        />

                        {/* APP ROUTES (Apenas Electron - ERP, CRM, PDV) */}
                        <Route 
                            path="/dashboard" 
                            element={
                                <ProtectedRoute>
                                    <DesktopOnlyRoute>
                                        <MainLayout>
                                            <Dashboard />
                                        </MainLayout>
                                    </DesktopOnlyRoute>
                                </ProtectedRoute>
                            } 
                        />

                        <Route 
                            path="/clients" 
                            element={
                                <ProtectedRoute>
                                    <DesktopOnlyRoute>
                                        <MainLayout>
                                            <ClientList />
                                        </MainLayout>
                                    </DesktopOnlyRoute>
                                </ProtectedRoute>
                            } 
                        />

                        <Route 
                            path="/pdv" 
                            element={
                                <ProtectedRoute>
                                    <DesktopOnlyRoute>
                                        <MainLayout>
                                            <div className="glass-panel" style={{ padding: '40px' }}>
                                                <h1 className="holographic-text">PDV System</h1>
                                                <p style={{ color: 'var(--text-secondary)', marginTop: '20px' }}>
                                                    Sistema de Ponto de Venda - Apenas disponível no App Desktop
                                                </p>
                                            </div>
                                        </MainLayout>
                                    </DesktopOnlyRoute>
                                </ProtectedRoute>
                            } 
                        />

                        <Route 
                            path="/crm" 
                            element={
                                <ProtectedRoute>
                                    <DesktopOnlyRoute>
                                        <MainLayout>
                                            <div className="glass-panel" style={{ padding: '40px' }}>
                                                <h1 className="holographic-text">CRM System</h1>
                                                <p style={{ color: 'var(--text-secondary)', marginTop: '20px' }}>
                                                    Sistema de Gestão de Clientes - Apenas disponível no App Desktop
                                                </p>
                                            </div>
                                        </MainLayout>
                                    </DesktopOnlyRoute>
                                </ProtectedRoute>
                            } 
                        />

                        {/* Fallback */}
                        <Route path="*" element={<Navigate to="/" />} />
                    </Routes>
                </AuthProvider>
            </LanguageProvider>
        </ThemeProvider>
    );
};

export default App;
