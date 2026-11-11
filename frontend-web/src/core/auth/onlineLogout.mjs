// Online session revocation. Empty body; local /logout is a different client.
export function onlineLogoutRequest(){
 return {method:'POST',url:'/auth/logout',data:undefined,transformRequest:[()=>undefined],validateStatus:(status)=>status===204||status===401};
}
export function interpretOnlineLogout(status){
 if(status===204||status===401)return 'closed';
 return 'uncertain';
}
