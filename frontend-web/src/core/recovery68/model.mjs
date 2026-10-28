// Recovery contains no service calls, operation retries or browser storage access.
export const release68 = 'frontend-consolidado68-20261026-1';
export function localPath68(path) {
 return typeof path === 'string' && (path === '/local' || path.startsWith('/local/'));
}
export function recovery68(path, notFound = false) {
 return Object.freeze({home:localPath68(path)?'/local/home':'/',code:notFound?'UI68-404':'UI68-RENDER',area:localPath68(path)?'local':'online'});
}
export function diagnostic68(info, now = new Date()) {
 // Deliberate allowlist: no errors, stacks, URLs, search params, IDs or credentials.
 if (!info || !['UI68-404','UI68-RENDER'].includes(info.code) || !['local','online'].includes(info.area) || !(now instanceof Date) || !Number.isFinite(now.getTime())) throw Error('invalid diagnostic');
 return JSON.stringify({release:release68,code:info.code,area:info.area,at:now.toISOString()},null,2);
}
export function protectRoutes68(routes, fallback) {
 // Preserve all guard elements, loaders, actions and route IDs by reference.
 // Every leaf owns its boundary: an error in a business view preserves its parent shell.
 return routes.map(route=>({...route,errorElement:route.errorElement??fallback,...(route.children?{children:protectRoutes68(route.children,fallback)}:{})}));
}
export function moveFocus68(container, activeBefore, currentActive, documentBody) {
 if (!container || (currentActive && currentActive!==activeBefore && currentActive!==documentBody)) return false;
 const heading=container.querySelector('h1');
 if (!heading) return false; // A lazy loading placeholder must not steal focus.
 const previous=heading.getAttribute('tabindex');
 heading.setAttribute('tabindex','-1');
 try {heading.focus({preventScroll:true});}
 catch {return false;}
 finally {if(previous===null)heading.removeAttribute('tabindex');else heading.setAttribute('tabindex',previous);}
 return true;
}
