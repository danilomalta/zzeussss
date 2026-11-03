import pt from '../../i18n/final108/pt-BR.json';
import en from '../../i18n/final108/en.json';
import es from '../../i18n/final108/es.json';
import fr from '../../i18n/final108/fr.json';
import zh from '../../i18n/final108/zh-CN.json';
import {words92} from './words92';
import {words97} from './words97';
export function wordsClose108(locale:string):Record<string,string>{return {...words92(locale),...words97(locale),...(locale.startsWith('en')?en:locale.startsWith('es')?es:locale.startsWith('fr')?fr:locale.startsWith('zh')?zh:pt)};}
