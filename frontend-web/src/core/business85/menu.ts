import labels from '../../i18n/business85/menu.json';
import type {Area85} from './types';
export function menuWords85(language:string):Record<Area85,string>{return (labels as Record<string,Record<Area85,string>>)[language]||labels['pt-BR'];}
