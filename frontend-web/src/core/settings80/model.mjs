export const defaults80=()=>({version:1,companyName:'',logoLight:'',logoDark:'',background:'',backgroundFit:'cover',mode:'theme',accent:'',lowEffects:false,compact:false,fiscal:{cnpj:'',ie:'',uf:'',regime:'',series:'1',nextNumber:'1'},equipment:{printer:'',paper:'80',copies:'1',connection:'system'}});
const text=(value,max)=>typeof value==='string'&&value.length<=max&&!/[\u0000-\u001f]/.test(value);
export function image80(value,limit){
 return value===''||(typeof value==='string'&&value.length<=limit&&/^data:image\/(png|jpeg|webp);base64,[A-Za-z0-9+/]+=*$/.test(value));
}
export function validate80(value){
 if(!value||Object.keys(value).some(key=>!Object.keys(defaults80()).includes(key))||value.version!==1)throw Error('settings');
 if(!text(value.companyName,100)||!image80(value.logoLight,700000)||!image80(value.logoDark,700000)||!image80(value.background,2800000)||!['cover','contain'].includes(value.backgroundFit)||!['theme','photo'].includes(value.mode)||(value.mode==='photo'&&!value.background)||!(value.accent===''||/^#[0-9a-f]{6}$/i.test(value.accent))||typeof value.lowEffects!=='boolean'||typeof value.compact!=='boolean')throw Error('appearance');
 const fiscal=value.fiscal,equipment=value.equipment;
 if(!fiscal||Object.keys(fiscal).sort().join()!==Object.keys(defaults80().fiscal).sort().join()||!text(fiscal.cnpj,18)||!text(fiscal.ie,24)||!text(fiscal.uf,2)||!text(fiscal.regime,20)||!/^[0-9]{1,3}$/.test(fiscal.series)||!/^[1-9][0-9]{0,8}$/.test(fiscal.nextNumber))throw Error('fiscal');
 if(!equipment||Object.keys(equipment).sort().join()!==Object.keys(defaults80().equipment).sort().join()||!text(equipment.printer,120)||!['58','80','A4'].includes(equipment.paper)||!['1','2','3'].includes(equipment.copies)||!['system','USB','network'].includes(equipment.connection))throw Error('equipment');
 return structuredClone(value);
}
export function settingsKey80(context){
 const fields=['tenant_id','store_id','identity_id','device_id'];
 if(!context||fields.some(key=>!text(context[key],128)||!context[key].trim()))throw Error('context');
 return JSON.stringify(fields.map(key=>context[key]));
}
const rgb=hex=>[1,3,5].map(index=>parseInt(hex.slice(index,index+2),16)/255);
function luminance(hex){return rgb(hex).map(c=>c<=.04045?c/12.92:((c+.055)/1.055)**2.4).reduce((sum,c,index)=>sum+c*[.2126,.7152,.0722][index],0);}
export function contrast80(a,b){const x=luminance(a),y=luminance(b);return (Math.max(x,y)+.05)/(Math.min(x,y)+.05);}
export function palette80(accent,dark){
 if(!/^#[0-9a-f]{6}$/i.test(accent))return null;
 const bg=dark?'#141417':'#ffffff';
 let color=accent;
 for(let count=0;count<24&&contrast80(color,bg)<3;count++){
  const channels=rgb(color).map(c=>Math.round((c*.85+(dark?.15:0))*255));
  color='#'+channels.map(c=>c.toString(16).padStart(2,'0')).join('');
 }
 return {accent:color,buttonText:contrast80(color,'#ffffff')>contrast80(color,'#000000')?'#ffffff':'#000000'};
}
