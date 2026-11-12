'use strict';
// Real Electron renderer and Chromium PDF pipeline, no database or fiscal provider.
const {app,BrowserWindow}=require('electron'),fs=require('node:fs'),path=require('node:path'),os=require('node:os'),assert=require('node:assert/strict');
const [stage,source,out,mode]=process.argv.slice(process.argv.findIndex(value=>path.resolve(value)===__filename)+1),root=fs.mkdtempSync(path.join(os.tmpdir(),'titan-print-test-'));
app.setPath('userData',root);app.disableHardwareAcceleration();fs.writeFileSync(path.join(root,'connection-v1.json'),JSON.stringify({version:1,mode:'local',operational_url:'http://127.0.0.1:9/local/v1',online_url:null}),{mode:0o600});
(async()=>{
 fs.mkdirSync(out,{recursive:false,mode:0o700});const main=require(path.join(stage,'main.cjs')),win=await main.ready;assert.equal(win.webContents.getURL(),'titan://app/local/login');
 const ts=require(path.join(source,'node_modules/typescript')),code=ts.transpileModule(fs.readFileSync(path.join(source,'src/core/local/print95.ts'),'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}).outputText;
 let requests=0;const server=require('node:http').createServer((_q,r)=>{requests++;r.end('unexpected')});await new Promise(r=>server.listen(0,'127.0.0.1',r));const probe='http://127.0.0.1:'+server.address().port+'/must-not-load';
 const results=[];
 for(const paper of ['58','80','a4']){
  let child;win.webContents.once('did-create-window',w=>child=w);
  const action=`(()=>{const exports={};${code};try{exports.print95('RECIBO DE TESTE',[['Total','31,98'],['Produto','Item sintetico <script>never()</script>']],'SEM VALOR FISCAL',{paper:${JSON.stringify(paper)},lang:'pt-BR',preview:true});return 'opened'}catch(e){return e.message}})()`;
  const status=await win.webContents.executeJavaScript(action,true);
  if(mode==='expect-block'){assert.equal(status,'popupBlocked');assert.equal(child,undefined);console.log(JSON.stringify({oldPopupBlocked:true,paper,physicalPrinting:false}));app.exit(0);return;}
  assert.equal(status,'opened');assert.ok(child);const prefs=child.webContents.getLastWebPreferences();assert.equal(prefs.javascript,false);assert.equal(prefs.sandbox,true);assert.equal(prefs.nodeIntegration,false);
  const pdf=await child.webContents.printToPDF({preferCSSPageSize:true,printBackground:true});assert.equal(pdf.subarray(0,5).toString(),'%PDF-');const box=/\/MediaBox\s*\[\s*0\s+0\s+([\d.]+)\s+([\d.]+)/.exec(pdf.toString('latin1'));assert.ok(box,'PDF page dimensions missing');assert.ok(Math.abs(Number(box[1])-(paper==='a4'?210:Number(paper))*72/25.4)<1,'PDF width differs from selected paper');fs.writeFileSync(path.join(out,'receipt-'+paper+'.pdf'),pdf,{mode:0o600,flag:'wx'});if(paper==='a4'){let refused=false;try{await child.webContents.loadURL(probe)}catch{refused=true;}assert.equal(refused,true);assert.equal(requests,0);}child.close();results.push({paper,pdfBytes:pdf.length,sandbox:true,javascript:false,networkRestricted:true});
 }
 await new Promise(r=>server.close(r));const report={networkProbeRequests:requests,realElectron:true,sourceModule:'frontend-web/src/core/local/print95.ts',realChromiumPDF:true,physicalPrinting:false,fiscalEmission:false,commercialData:false,results};fs.writeFileSync(path.join(out,'print.json'),JSON.stringify(report,null,2),{mode:0o600,flag:'wx'});console.log(JSON.stringify(report));app.exit(0);
})().catch(error=>{console.error(error.message);app.exit(1)});
