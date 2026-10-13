export function logoDestination({pathname,localActive,onlineActive}){
 if(typeof pathname!=='string')throw new Error('Invalid location');
 if(pathname==='/local'||pathname.startsWith('/local/'))return localActive?'/local/home':'/local/login';
 if(onlineActive)return '/';
 return localActive?'/local/home':'/local/login';
}
