import { Routes, Route, Navigate } from 'react-router-dom'
import { useState } from 'react'
import { ThemeProvider } from './contexts/ThemeContext.jsx'
import { LanguageProvider } from './contexts/LanguageContext.jsx'
import { AuthProvider } from './contexts/AuthContext.jsx'
import Header from './components/layout/Header.jsx'
import SideDrawer from './components/layout/SideDrawer.jsx'
import Footer from './components/layout/Footer.jsx'
import LandingPage from './pages/LandingPage.jsx'
import AuthPage from './pages/AuthPage.jsx'
import Onboarding from './pages/Onboarding.jsx'

function App() {
  const [open, setOpen] = useState(false)
  return (
    <LanguageProvider>
      <ThemeProvider>
        <AuthProvider>
          <div className="app-root">
            <SideDrawer open={open} onClose={() => setOpen(false)} />
            <div className={`page-push-wrapper ${open ? 'drawer-open' : ''}`}>
            <Header onMenuClick={() => setOpen(true)} />
            <main>
              <Routes>
                <Route path="/" element={<LandingPage />} />
                <Route path="/auth" element={<AuthPage />} />
                <Route path="/onboarding" element={<Onboarding />} />
                <Route path="*" element={<Navigate to="/" replace />} />
              </Routes>
            </main>
            <Footer />
            </div>
          </div>
        </AuthProvider>
      </ThemeProvider>
    </LanguageProvider>
  )
}

export default App
