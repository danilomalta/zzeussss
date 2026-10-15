import labels from '../../i18n/operations83/menu.json';
export function menuLabel83(language:string){return labels[language.split('-')[0] as keyof typeof labels]||labels.pt;}
