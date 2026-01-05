import React from 'react';
import { useLanguage } from '../../context/LanguageContext';

const languages = [
    { code: 'pt', flag: '🇧🇷', name: 'Português' },
    { code: 'en', flag: '🇺🇸', name: 'English' },
    { code: 'es', flag: '🇪🇸', name: 'Español' },
    { code: 'zh', flag: '🇨🇳', name: '中文' },
    { code: 'hi', flag: '🇮🇳', name: 'हिन्दी' },
    { code: 'ar', flag: '🇸🇦', name: 'العربية' },
    { code: 'fr', flag: '🇫🇷', name: 'Français' },
    { code: 'ru', flag: '🇷🇺', name: 'Русский' },
] as const;

const LanguageSwitcher: React.FC = () => {
    const { lang, setLang } = useLanguage();
    const currentLang = languages.find(l => l.code === lang) || languages[0];

    return (
        <div className="language-switcher" style={{ position: 'fixed', bottom: '20px', right: '20px', zIndex: 99999 }}>
            <button className="language-btn" aria-label="Selecionar idioma">
                {currentLang.flag}
            </button>
            <div className="language-dropdown">
                {languages.map((language) => (
                    <div
                        key={language.code}
                        className={`language-option ${lang === language.code ? 'active' : ''}`}
                        onClick={() => setLang(language.code as any)}
                    >
                        <span style={{ fontSize: '1.2rem' }}>{language.flag}</span>
                        <span>{language.name}</span>
                    </div>
                ))}
            </div>
        </div>
    );
};

export default LanguageSwitcher;

