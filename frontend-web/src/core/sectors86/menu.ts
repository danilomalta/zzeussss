import labels from '../../i18n/sectors86/menu.json';
import type {Area86} from './types';
export function menuWords86(language:string):Record<Area86,string>{return (labels as Record<string,Record<Area86,string>>)[language]||labels['pt-BR'];}
