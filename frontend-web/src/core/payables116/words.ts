import pt from "../../i18n/payables116/pt-BR.json";import en from "../../i18n/payables116/en.json";import es from "../../i18n/payables116/es.json";import fr from "../../i18n/payables116/fr.json";import zh from "../../i18n/payables116/zh-CN.json";
import {words113} from "../payables113/words";
export function words116(locale:string):Record<string,string>{return {...words113(locale),...(locale.startsWith("en")?en:locale.startsWith("es")?es:locale.startsWith("fr")?fr:locale.startsWith("zh")?zh:pt),confirmPhysical:words113(locale).confirm};}
