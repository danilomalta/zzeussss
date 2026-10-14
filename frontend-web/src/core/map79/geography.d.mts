export interface Municipality79{id:string;uf:string;name:string;path:string;bounds:[number,number,number,number];}
export const stateCodes:Record<string,string>;
export function projectCoordinate(longitude:number,latitude:number):[number,number];
export function municipalities(uf:string,names:unknown,geo:unknown):Municipality79[];
export function validCachedRows(uf:string,rows:unknown):rows is Municipality79[];
