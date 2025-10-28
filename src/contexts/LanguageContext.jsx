import { createContext, useContext, useEffect, useMemo, useState } from 'react'
import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import en from '../locales/en.json'
import pt from '../locales/pt.json'

// Initialize i18n once
if (!i18n.isInitialized) {
  i18n.use(initReactI18next).init({
    resources: {
      en: { translation: en },
      pt: { translation: pt },
    },
    lng: typeof window !== 'undefined' ? localStorage.getItem('lang') || 'pt' : 'pt',
    fallbackLng: 'en',
    interpolation: { escapeValue: false },
  })
}

const LanguageContext = createContext({ lang: 'pt', setLang: () => {} })

export function LanguageProvider({ children }) {
  const [lang, setLangState] = useState(i18n.language)

  const setLang = (next) => {
    i18n.changeLanguage(next)
    localStorage.setItem('lang', next)
    setLangState(next)
  }

  useEffect(() => {
    document.documentElement.setAttribute('lang', lang)
  }, [lang])

  const value = useMemo(() => ({ lang, setLang }), [lang])

  return <LanguageContext.Provider value={value}>{children}</LanguageContext.Provider>
}

export function useLanguage() {
  return useContext(LanguageContext)
}
