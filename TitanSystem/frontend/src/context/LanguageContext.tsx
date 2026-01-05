import { createContext, useContext, useEffect, useMemo, useState, ReactNode } from 'react';
import { translations, useTranslation as getTranslation } from '../utils/translations';

type Language = 'pt' | 'en' | 'es' | 'zh' | 'hi' | 'ar' | 'fr' | 'ru';

interface LanguageContextData {
    lang: Language;
    setLang: (lang: Language) => void;
    t: (key: keyof typeof translations.pt) => string;
}

const LanguageContext = createContext<LanguageContextData>({} as LanguageContextData);

const SUPPORTED_LANGUAGES: Language[] = ['pt', 'en', 'es', 'zh', 'hi', 'ar', 'fr', 'ru'];

export const LanguageProvider = ({ children }: { children: ReactNode }) => {
    const [lang, setLangState] = useState<Language>(() => {
        const stored = localStorage.getItem('lang');
        return (SUPPORTED_LANGUAGES.includes(stored as Language)) ? stored as Language : 'pt';
    });

    useEffect(() => {
        document.documentElement.setAttribute('lang', lang);
        localStorage.setItem('lang', lang);
        // Força re-render de componentes que usam traduções
        window.dispatchEvent(new Event('language-changed'));
    }, [lang]);

    const setLang = (next: Language) => {
        if (SUPPORTED_LANGUAGES.includes(next)) {
            setLangState(next);
        }
    };

    const t = getTranslation(lang);

    const value = useMemo(() => ({ lang, setLang, t }), [lang]);

    return <LanguageContext.Provider value={value}>{children}</LanguageContext.Provider>;
};

export const useLanguage = () => {
    const context = useContext(LanguageContext);
    if (!context) {
        throw new Error('useLanguage must be used within a LanguageProvider');
    }
    return context;
};
