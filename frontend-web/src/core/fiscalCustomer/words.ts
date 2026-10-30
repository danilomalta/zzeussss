import pt from '../../i18n/fiscalCustomer/pt-BR.json';
import en from '../../i18n/fiscalCustomer/en.json';
import es from '../../i18n/fiscalCustomer/es.json';
import fr from '../../i18n/fiscalCustomer/fr.json';
import zh from '../../i18n/fiscalCustomer/zh-CN.json';
export function fiscalWords(locale:string):Record<string,string> {
  return locale.startsWith('en') ? en : locale.startsWith('es') ? es : locale.startsWith('fr') ? fr : locale.startsWith('zh') ? zh : pt;
}
