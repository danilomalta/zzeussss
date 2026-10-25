// UI identity only: never use this counter as a commercial operation ID.
// Weak keys keep login instances separate without putting tokens into React keys.
const views=new WeakMap();let sequence=0;
export function posViewIdentity(session,token){
 if(!session||typeof session!=='object'||typeof token!=='string'||!token)throw Error('Invalid POS view identity');
 let value=views.get(session);if(!value||value.token!==token){value={token,id:'pos-login-'+(++sequence)};views.set(session,value);}return value.id;
}
