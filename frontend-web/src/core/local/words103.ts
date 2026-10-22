import pt from '../../i18n/workspace103/pt-BR.json';import en from '../../i18n/workspace103/en.json';import es from '../../i18n/workspace103/es.json';import fr from '../../i18n/workspace103/fr.json';import zh from '../../i18n/workspace103/zh-CN.json';import {words100} from './words100';
import {words98} from './words98';
export function words103(l:string):Record<string,string>{return {...words100(l),...words98(l),...(l.startsWith('en')?en:l.startsWith('es')?es:l.startsWith('fr')?fr:l.startsWith('zh')?zh:pt)};}
