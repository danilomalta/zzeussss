import { useLanguage } from '../../contexts/LanguageContext.jsx'

export default function LanguageSwitcher() {
  const { lang, setLang } = useLanguage()
  const next = lang === 'pt' ? 'en' : 'pt'
  return (
    <button className="icon-button" aria-label="Switch language" onClick={() => setLang(next)}>
      🌐 {lang.toUpperCase()}
    </button>
  )
}
