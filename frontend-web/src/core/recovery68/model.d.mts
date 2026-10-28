import type {RouteObject} from 'react-router-dom';
export const release68:string;
export interface RecoveryInfo68{home:string;code:'UI68-404'|'UI68-RENDER';area:'local'|'online'}
export function localPath68(path:unknown):boolean;
export function recovery68(path:unknown,notFound?:boolean):Readonly<RecoveryInfo68>;
export function diagnostic68(info:RecoveryInfo68,now?:Date):string;
export function protectRoutes68(routes:RouteObject[],fallback:React.ReactNode):RouteObject[];
export function moveFocus68(container:HTMLElement|null,activeBefore:Element|null,currentActive:Element|null,documentBody:HTMLElement):boolean;
