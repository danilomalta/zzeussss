let pending=null;
export function stageReport76(context,id,name,matrix,now=Date.now()){
 if(typeof context!=='string'||!context||typeof id!=='string'||!id||typeof name!=='string'||!Array.isArray(matrix)||matrix.length>501||matrix.some(r=>!Array.isArray(r)||r.length>20||r.some(c=>!['number','string'].includes(typeof c))))throw Error('report');
 pending={context,id,name,matrix:structuredClone(matrix),expires:now+60000};
}
export function clearReport76(){pending=null;}
export function takeReport76(context,id,now=Date.now()){if(!pending)return null;const p=pending;pending=null;return p.context===context&&p.id===id&&p.expires>now?{name:p.name,matrix:p.matrix}:null;}
export function reportBook76(base,report){const book=structuredClone(base),id=crypto.randomUUID(),cells={};report.matrix.forEach((row,r)=>{cells[r]=Object.fromEntries(row.map((v,c)=>[c,{v,t:typeof v==='number'?2:1}]));});book.id=crypto.randomUUID();book.name=report.name;book.sheetOrder=[id];book.sheets={[id]:{id,name:report.name.replace(/[\[\]\/\\?*:]/g,' ').slice(0,31),rowCount:Math.max(1000,report.matrix.length+20),columnCount:50,cellData:cells}};book.resources=[];book.styles={};return book;}
