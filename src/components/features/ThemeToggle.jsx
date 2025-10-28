import { useTheme } from '../../contexts/ThemeContext.jsx'

export default function ThemeToggle() {
  const { theme, toggleTheme } = useTheme()
  return (
    <button aria-label={theme === 'light' ? 'Dark Mode' : 'Light Mode'} className="icon-button" onClick={toggleTheme}>
      {theme === 'light' ? '🌙' : '☀️'}
    </button>
  )
}
