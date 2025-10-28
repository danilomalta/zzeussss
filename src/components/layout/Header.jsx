import { useState } from 'react'
import Logo from '../common/Logo.jsx'
import ThemeToggle from '../features/ThemeToggle.jsx'
import LanguageSwitcher from '../features/LanguageSwitcher.jsx'
import SideDrawer from './SideDrawer.jsx'

export default function Header() {
  const [open, setOpen] = useState(false)
  return (
    <header className="header">
      <div className="container header-inner">
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <button className="icon-button" aria-label="Menu" onClick={() => setOpen(true)}>☰</button>
          <Logo />
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <LanguageSwitcher />
          <ThemeToggle />
        </div>
      </div>
      <SideDrawer open={open} onClose={() => setOpen(false)} />
    </header>
  )
}
