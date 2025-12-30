import { Routes, Route, Navigate } from 'react-router-dom'
import { useState, useEffect } from 'react'
import { ThemeProvider } from './contexts/ThemeContext.jsx'
import { LanguageProvider } from './contexts/LanguageContext.jsx'
import { AuthProvider } from './contexts/AuthContext.jsx'
import { ToastProvider } from './contexts/ToastContext.jsx'
import ErrorBoundary from './components/common/ErrorBoundary.jsx'
import Header from './components/layout/Header.jsx'
import SideDrawer from './components/layout/SideDrawer.jsx'
import Footer from './components/layout/Footer.jsx'
import LandingPage from './pages/LandingPage.jsx'
import AuthPage from './pages/AuthPage.jsx'
import Onboarding from './pages/Onboarding.jsx'
import ProtectedRoute from './components/common/ProtectedRoute.jsx'

function App() {
  const [open, setOpen] = useState(false)

  // Close drawer on ESC key
  useEffect(() => {
    const handleEscape = (e) => {
      if (e.key === 'Escape' && open) {
        setOpen(false)
      }
    }
    document.addEventListener('keydown', handleEscape)
    return () => document.removeEventListener('keydown', handleEscape)
  }, [open])

  // Prevent body scroll when drawer is open
  useEffect(() => {
    if (open) {
      document.body.style.overflow = 'hidden'
    } else {
      document.body.style.overflow = ''
    }
    return () => {
      document.body.style.overflow = ''
    }
  }, [open])

  return (
    <ErrorBoundary>
      <LanguageProvider>
        <ThemeProvider>
          <ToastProvider>
            <AuthProvider>
              <div className="app-root">
                <SideDrawer open={open} onClose={() => setOpen(false)} />
                {open && (
                  <div
                    className="drawer-backdrop"
                    onClick={() => setOpen(false)}
                    style={{
                      position: 'fixed',
                      inset: 0,
                      background: 'rgba(0,0,0,0.5)',
                      zIndex: 9,
                      animation: 'fadeIn 0.2s ease-out',
                    }}
                    aria-hidden="true"
                  />
                )}
                <div className={`page-push-wrapper ${open ? 'drawer-open' : ''}`}>
                  <Header onMenuClick={() => setOpen(true)} />
                  <main>
                    <Routes>
                      <Route path="/" element={<LandingPage />} />
                      <Route path="/auth" element={<AuthPage />} />
                      <Route
                        path="/onboarding"
                        element={
                          <ProtectedRoute>
                            <Onboarding />
                          </ProtectedRoute>
                        }
                      />
                      <Route path="*" element={<Navigate to="/" replace />} />
                    </Routes>
                  </main>
                  <Footer />
                </div>
              </div>
            </AuthProvider>
          </ToastProvider>
        </ThemeProvider>
      </LanguageProvider>
    </ErrorBoundary>
  )
}

export default App
