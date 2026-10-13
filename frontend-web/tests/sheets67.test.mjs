import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createWorkbook,createSheet,defaultCell,evaluateSheet,parseWorkbook,parseDelimited,csvText,sortSheet} from '../src/core/local/sheetModel.mjs';
import {allowedAreas,areaForRoute} from '../src/core/local/accessModel.mjs';
const sheet=(values)=>{const result=createSheet('id','Aba');for(const [key,value] of Object.entries(values))result.cells[key]={...defaultCell,value};return result;};
test('fórmulas aritméticas respeitam prioridade, referências, percentuais e arredondamento',()=>{
 const result=evaluateSheet(sheet({A1:'10',A2:'2,5',B1:'=A1+A2*2',B2:'=(A1+A2)*2',B3:'=-2^2',B4:'=50%*A1',B5:'=ROUND(1.005,2)',B6:'=ROUND(-1.5,0)'}));
 assert.equal(result.B1,15);assert.equal(result.B2,25);assert.equal(result.B3,-4);assert.equal(result.B4,5);assert.equal(result.B5,1.01);assert.equal(result.B6,-2);
});
test('intervalos e funções portuguesas/inglesas calculam dados reais digitados',()=>{
 const result=evaluateSheet(sheet({A1:'10',A2:'20',A3:'texto',B1:'=SOMA(A1:A3)',B2:'=MÉDIA(A1:A3)',B3:'=MIN(A1:A3)',B4:'=MAX(A1:A3)',B5:'=COUNT(A1:A3)',B6:'=ABS(-5)'}));
 assert.equal(result.B1,30);assert.equal(result.B2,15);assert.equal(result.B3,10);assert.equal(result.B4,20);assert.equal(result.B5,2);assert.equal(result.B6,5);
});
test('SE/IF escolhe apenas o ramo usado e preserva texto sem execução',()=>{
 const result=evaluateSheet(sheet({A1:'1',B1:'=IF(A1>0,"Sim",1/0)',B2:'=SE(A1<0;1/0;"Não")',B3:'="<script>alert(1)</script>"'}));
 assert.equal(result.B1,'Sim');assert.equal(result.B2,'Não');assert.equal(result.B3,'<script>alert(1)</script>');
});
test('ciclos, divisão por zero, referências e funções externas são recusados',()=>{
 const result=evaluateSheet(sheet({A1:'=B1',B1:'=A1',A2:'=1/0',A3:'=Z999',A4:'=WEBSERVICE("https://example.com")',A5:'=alert(1)',A6:'=1+!',A7:'=SUM('}));
 assert.equal(result.A1,'#CYCLE!');assert.equal(result.B1,'#CYCLE!');assert.equal(result.A2,'#DIV/0!');assert.equal(result.A3,'#REF!');assert.equal(result.A4,'#NAME?');assert.equal(result.A5,'#NAME?');assert.equal(result.A6,'#FORMULA!');assert.equal(result.A7,'#FORMULA!');
});
test('arquivo Titan preserva fórmulas, abas e formatos sem incorporar HTML executável ou campos secretos',()=>{
 const book=createWorkbook('id','Teste','Aba');book.sheets[0]=sheet({A1:'=1+2'});book.sheets[0].cells.A1.bold=true;
 const result=parseWorkbook(JSON.stringify({...book,token:'secret'}));assert.equal(result.sheets[0].cells.A1.value,'=1+2');assert.equal(result.sheets[0].cells.A1.bold,true);assert.ok(!('token' in result));
 const original=JSON.stringify(book);for(const mutate of [value=>value.sheets[0].rows=101,value=>value.sheets[0].cols=27,value=>value.sheets.push(value.sheets[0]),value=>value.sheets[0].cells.A1.format='html',value=>value.sheets[0].cells.A01={...defaultCell,value:'hidden'}]){const invalid=JSON.parse(original);mutate(invalid);assert.throws(()=>parseWorkbook(JSON.stringify(invalid)));}
 assert.throws(()=>parseWorkbook('x'.repeat(1000001)));
});
test('CSV e colagem respeitam aspas, linhas, BOM e limites',()=>{
 assert.deepEqual(parseDelimited('a\tb\n1\t2'),[['a','b'],['1','2']]);
 assert.deepEqual(parseDelimited('\uFEFF"a;b",c\r\n"linha\n2","aspas ""duplas"""'),[['a;b','c'],['linha\n2','aspas "duplas"']]);
 assert.deepEqual(parseDelimited('"a";"b"\r\n"1";"2"'),[['a','b'],['1','2']]);
 assert.throws(()=>parseDelimited('"sem fim'));assert.throws(()=>parseDelimited(Array(28).fill('x').join('\t')));
 const exported=csvText([['=WEBSERVICE("x")','@x','a;b'],[-5,10,'normal']]);assert.ok(exported.includes("'=WEBSERVICE"));assert.ok(exported.includes("'@x"));assert.ok(exported.includes('"-5"'));
});
test('ordenação preserva cabeçalho, linha completa e estilos, e não corrompe fórmulas',()=>{
 const original=sheet({A1:'Nome',B1:'Valor',A2:'B',B2:'20',A3:'A',B3:'10'});original.cells.B3.bold=true;
 const result=sortSheet(original,1);assert.equal(result.cells.A1.value,'Nome');assert.equal(result.cells.A2.value,'A');assert.equal(result.cells.B2.value,'10');assert.equal(result.cells.B2.bold,true);assert.equal(original.cells.B2.value,'20');
 assert.equal(sortSheet(original,1,true).cells.B2.value,'20');assert.throws(()=>sortSheet(sheet({A1:'=1+2'}),0));
});
test('planilha utilitária exige contexto autenticado sem liberar módulos comerciais',()=>{
 assert.equal(areaForRoute('/local/sheets'),'sheets');assert.ok(!allowedAreas(null).includes('sheets'));
 const areas=allowedAreas({identity_id:'local-user',role:'employee',permissions:[],license:{state:'not_installed',modules:[]}});assert.ok(areas.includes('sheets'));assert.ok(!areas.includes('pos'));assert.ok(!areas.includes('hr'));
});
test('cinco idiomas informam limites de arquivos, cálculo e rascunhos',()=>{
 const copies=['pt-BR','en','es','fr','zh-CN'].map(lang=>JSON.parse(readFileSync(new URL(`../src/i18n/sheets/${lang}.json`,import.meta.url))));
 for(const copy of copies){assert.deepEqual(Object.keys(copy),Object.keys(copies[0]));for(const value of Object.values(copy))assert.ok(value.trim());assert.ok(copy.limits&&copy.draftHelp&&copy.sortError&&copy.leaveConfirm);}
});
