import test from 'node:test';
import assert from 'node:assert/strict';
import ExcelJS from 'exceljs';
import JSZip from 'jszip';
import {emptySnapshot,validateSnapshot,readTitanFile,writeTitanFile,legacyToSnapshot,csvToSnapshot,snapshotCSV,snapshotToXLSX,xlsxToSnapshot,pivotRows} from '../src/core/local/sheetFiles68.mjs';
import {createWorkbook,defaultCell} from '../src/core/local/sheetModel.mjs';
const bytes=buffer=>buffer.buffer.slice(buffer.byteOffset,buffer.byteOffset+buffer.byteLength);
test('Titan preserva recursos avançados e rejeita limites e coordenadas inválidas',()=>{
 const b=emptySnapshot();b.resources=[{name:'SHEET_DATA_VALIDATION_PLUGIN',data:'{"rules":[]}'}];b.sheets['sheet-1'].cellData={0:{0:{v:12,t:2,f:'=SUM(6,6)',s:{bl:1}}}};
 assert.deepEqual(readTitanFile(writeTitanFile(b)),b);
 for(const change of [x=>x.sheets['sheet-1'].rowCount=10001,x=>x.sheets['sheet-1'].cellData={1000:{0:{v:1}}},x=>x.sheets['sheet-1'].mergeData=[{startRow:0,endRow:50000,startColumn:0,endColumn:1}],x=>x.sheets['sheet-1'].cellData[0][0].v=Infinity]){const c=structuredClone(b);change(c);assert.throws(()=>validateSnapshot(c));}
 assert.throws(()=>readTitanFile('{"format":"unknown"}'));
});
test('arquivos Titan67 migram valores, fórmulas e formatação',()=>{
 const b=createWorkbook('old','Anterior','Dados');b.sheets[0].cells={A1:{...defaultCell,value:'10',bold:true},A2:{...defaultCell,value:'=SOMA(A1,2)'},B1:{...defaultCell,value:'Produto'}};
 const converted=legacyToSnapshot(JSON.stringify(b));const s=converted.sheets[converted.sheetOrder[0]];assert.equal(s.cellData[0][0].v,10);assert.equal(s.cellData[0][0].s.bl,1);assert.equal(s.cellData[1][0].f,'=SUM(A1,2)');
});
test('CSV importa mais de100 linhas e26 colunas e exporta texto sem executar fórmulas',()=>{
 const text=Array.from({length:150},()=>Array(40).fill('12').join(';')).join('\n');const b=csvToSnapshot(text,'CSV');assert.equal(b.sheets['sheet-1'].cellData[149][39].v,12);b.sheets['sheet-1'].cellData[0][0]={v:'=WEBSERVICE("x")'};assert.ok(snapshotCSV(b).includes("'=WEBSERVICE"));
});
test('XLSX round trip preserva múltiplas abas, números, fórmula, estilo e mesclagem',async()=>{
 const b=emptySnapshot('Custos'),s=b.sheets['sheet-1'];s.cellData={0:{0:{v:'Produto',s:{bl:1,bg:{rgb:'#eeeeee'}}},1:{v:10,t:2,s:{n:{pattern:'"R$" #,##0.00'}}}},1:{1:{f:'=B1*2',v:20,t:2}},3:{0:{v:'Mescla'}}};s.mergeData=[{startRow:3,endRow:3,startColumn:0,endColumn:1}];b.sheetOrder.push('s2');b.sheets.s2={id:'s2',name:'Extra',rowCount:1000,columnCount:50,cellData:{0:{0:{v:true,t:3}}}};
 const output=await snapshotToXLSX(b);const read=await xlsxToSnapshot(bytes(output),'Custos');assert.equal(read.sheetOrder.length,2);const target=read.sheets[read.sheetOrder[0]];assert.equal(target.cellData[0][1].v,10);assert.equal(target.cellData[1][1].f,'=B1*2');assert.equal(target.cellData[1][1].v,20);assert.equal(target.cellData[0][0].s.bl,1);assert.equal(target.mergeData[0].endColumn,1);assert.equal(read.sheets[read.sheetOrder[1]].cellData[0][0].v,true);
});
test('XLSX trata datas e resultado de erro sem inserir objetos como valores',async()=>{
 const w=new ExcelJS.Workbook(),s=w.addWorksheet('Data');s.getCell('A1').value=new Date('2026-01-01T00:00:00Z');s.getCell('A2').value={formula:'1/0',result:{error:'#DIV/0!'}};const b=await xlsxToSnapshot(bytes(await w.xlsx.writeBuffer()),'Data');assert.equal(typeof b.sheets[b.sheetOrder[0]].cellData[0][0].v,'number');assert.equal(b.sheets[b.sheetOrder[0]].cellData[1][0].f,'=1/0');
});
test('XLSX recusa macros e pacote com expansão acima de30MB',async()=>{
 const z=new JSZip();z.file('xl/vbaProject.bin','macro');
 const macro=await z.generateAsync({type:'nodebuffer'});await assert.rejects(()=>xlsxToSnapshot(bytes(macro),'Macro'));
 const large=new JSZip();large.file('xl/worksheets/sheet1.xml','x'.repeat(30000001));const compressed=await large.generateAsync({type:'nodebuffer',compression:'DEFLATE'});await assert.rejects(()=>xlsxToSnapshot(bytes(compressed),'Grande'));
});
test('resumo soma e conta por categoria sem alterar a fonte',()=>{const b=emptySnapshot();b.sheets['sheet-1'].cellData={0:{0:{v:'Tipo'},1:{v:'Valor'}},1:{0:{v:'A'},1:{v:10}},2:{0:{v:'B'},1:{v:3}},3:{0:{v:'A'},1:{v:7}}};const original=JSON.stringify(b);assert.deepEqual(pivotRows(b),[{label:'A',sum:17,count:2},{label:'B',sum:3,count:1}]);assert.equal(JSON.stringify(b),original);});

test('migração de fórmulas preserva texto entre aspas e funções com sufixo SE',()=>{const b=createWorkbook('old','Antes','Dados');b.sheets[0].cells={A1:{...defaultCell,value:'=SE(1;"a;b";FALSE())'}};const result=legacyToSnapshot(JSON.stringify(b));assert.equal(result.sheets[result.sheetOrder[0]].cellData[0][0].f,'=IF(1,"a;b",FALSE())');});
test('CSV conserva zeros iniciais e inteiros que perderiam precisão',()=>{const b=csvToSnapshot('Código;Valor\n00123;9007199254740993','Dados');assert.equal(b.sheets['sheet-1'].cellData[1][0].v,'00123');assert.equal(b.sheets['sheet-1'].cellData[1][1].v,'9007199254740993');});
