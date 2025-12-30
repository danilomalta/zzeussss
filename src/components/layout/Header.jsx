import Logo from '../common/Logo.jsx'
import ThemeToggle from '../features/ThemeToggle.jsx'
import LanguageSwitcher from '../features/LanguageSwitcher.jsx'

export default function Header({ onMenuClick }) {
  return (
    <header className="header">
      <div className="container header-inner">
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <button className="icon-button" aria-label="Menu" onClick={onMenuClick}>☰</button>
          <Logo />
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <LanguageSwitcher />
          <ThemeToggle />
        </div>
      </div>
    </header>
  )
}
