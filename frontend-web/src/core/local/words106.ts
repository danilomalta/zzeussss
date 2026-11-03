import pt from '../../i18n/paper106/pt-BR.json';
import en from '../../i18n/paper106/en.json';
import es from '../../i18n/paper106/es.json';
import fr from '../../i18n/paper106/fr.json';
import zh from '../../i18n/paper106/zh-CN.json';
import {words96} from './words96';
export function words106(locale:string):Record<string,string>{return {...words96(locale),...(locale.startsWith('en')?en:locale.startsWith('es')?es:locale.startsWith('fr')?fr:locale.startsWith('zh')?zh:pt)};}
