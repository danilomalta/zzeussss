import pt from '../../i18n/sectors86/pt-BR.json';
import en from '../../i18n/sectors86/en.json';
import es from '../../i18n/sectors86/es.json';
import fr from '../../i18n/sectors86/fr.json';
import zh from '../../i18n/sectors86/zh-CN.json';
export function dictionary86(language:string):Record<string,unknown>{return language.startsWith('en')?en:language.startsWith('es')?es:language.startsWith('fr')?fr:language.startsWith('zh')?zh:pt;}
