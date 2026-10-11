export interface Plan {id:string; name:string; price:number; previous:number; description:string}
export const plans: Plan[];
export function recommendedModule(activity:string):string|null;
export function planTotals(selected:string[]):{monthly:number;savings:number};
export function formatMoney(cents:number,locale?:string):string;
