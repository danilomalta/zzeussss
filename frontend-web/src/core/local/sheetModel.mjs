export const MAX_ROWS=100,MAX_COLS=26,MAX_SHEETS=8;
export const defaultCell=Object.freeze({value:'',bold:false,italic:false,align:'left',format:'general',fill:'none'});
export const cellKey=(row,col)=>String.fromCharCode(65+col)+(row+1);
export function createSheet(id,name){return {id,name,rows:30,cols:12,cells:{},freeze:true};}
export function createWorkbook(id,title,name){return {schema:1,title,sheets:[createSheet(id,name)]};}
const aliases={SOMA:'SUM',MEDIA:'AVERAGE','MÉDIA':'AVERAGE',MINIMO:'MIN',MÍNIMO:'MIN',MAXIMO:'MAX',MÁXIMO:'MAX',CONTAR:'COUNT',CONT:'COUNT',SE:'IF',ARRED:'ROUND',ABS:'ABS'};
class FormulaError extends Error{}
const fail=code=>{throw new FormulaError(code);};
function parseFormula(input){
 if(input.length>2000)fail('#LIMIT!');
 const tokens=input.match(/"(?:[^"]|"")*"|\$?[A-Za-z]\$?\d+|[A-Za-zÀ-ÿ_.]+|\d+(?:\.\d+)?|>=|<=|<>|[+\-*/^%(),;:<>=]/g)||[];
 if(tokens.join('').replace(/\s/g,'')!==input.replace(/\s/g,''))fail('#FORMULA!');
 let index=0,depth=0;const peek=()=>tokens[index],take=()=>tokens[index++];
 function expression(min=0){if(++depth>64)fail('#LIMIT!');let node=primary();
  const precedence={'=':1,'<>':1,'<':1,'>':1,'<=':1,'>=':1,'+':2,'-':2,'*':3,'/':3,'^':4};
  while(peek() in precedence&&precedence[peek()]>=min){const op=take();node={type:'op',op,left:node,right:expression(precedence[op]+(op==='^'?0:1))};}
  depth--;return node;
 }
 function primary(){const token=take();if(!token)fail('#FORMULA!');let node;
  if(token==='+'||token==='-')node={type:'unary',op:token,value:expression(4)};
  else if(token==='('){node=expression();if(take()!==')')fail('#FORMULA!');}
  else if(/^\d/.test(token))node={type:'number',value:Number(token)};
  else if(token.startsWith('"'))node={type:'string',value:token.slice(1,-1).replace(/""/g,'"')};
  else if(/^\$?[a-z]\$?\d+$/i.test(token)){node={type:'ref',key:token.replace(/\$/g,'').toUpperCase()};if(peek()===':'){take();const end=take();if(!/^\$?[a-z]\$?\d+$/i.test(end||''))fail('#REF!');node={type:'range',start:node.key,end:end.replace(/\$/g,'').toUpperCase()};}}
  else {const name=aliases[token.toUpperCase()]||token.toUpperCase();if(take()!=='(')fail('#NAME?');const args=[];if(peek()!==')')do{args.push(expression());if(![',',';'].includes(peek()))break;take();}while(true);if(take()!==')')fail('#FORMULA!');node={type:'call',name,args};}
  while(peek()==='%'){take();node={type:'percent',value:node};}return node;
 }
 const node=expression();if(index!==tokens.length)fail('#FORMULA!');return node;
}
export function evaluateSheet(sheet){
 const result={},memo=new Map(),visiting=new Set();let work=0;
 function position(key){const match=/^([A-Z])(\d+)$/.exec(key);if(!match)fail('#REF!');const row=Number(match[2])-1,col=match[1].charCodeAt(0)-65;if(row<0||row>=sheet.rows||col<0||col>=sheet.cols)fail('#REF!');return {row,col};}
 const number=value=>typeof value==='number'?value:typeof value==='boolean'?(value?1:0):value===''?0:fail('#VALUE!');
 function visit(key){position(key);if(memo.has(key))return memo.get(key);if(visiting.has(key))fail('#CYCLE!');if(visiting.size>=64||++work>20000)fail('#LIMIT!');visiting.add(key);
  try{const raw=sheet.cells[key]?.value||'';let value;if(raw.startsWith('='))value=calc(parseFormula(raw.slice(1)));else if(/^-?\d+(?:[.,]\d+)?$/.test(raw.trim()))value=Number(raw.trim().replace(',','.'));else value=raw;
   if(typeof value==='number'&&!Number.isFinite(value))fail('#NUM!');if(Array.isArray(value))fail('#VALUE!');memo.set(key,value);return value;
  }finally{visiting.delete(key);}
 }
 function calc(node){
  if(++work>20000)fail('#LIMIT!');
  if(node.type==='number'||node.type==='string')return node.value;
  if(node.type==='ref')return visit(node.key);
  if(node.type==='range'){const a=position(node.start),b=position(node.end),list=[];for(let row=Math.min(a.row,b.row);row<=Math.max(a.row,b.row);row++)for(let col=Math.min(a.col,b.col);col<=Math.max(a.col,b.col);col++)list.push(visit(cellKey(row,col)));return list;}
  if(node.type==='unary')return (node.op==='-'?-1:1)*number(calc(node.value));
  if(node.type==='percent')return number(calc(node.value))/100;
  if(node.type==='op'){const left=calc(node.left),right=calc(node.right);if(['=','<>','<','>','<=','>='].includes(node.op))return {'=':left===right,'<>':left!==right,'<':left<right,'>':left>right,'<=':left<=right,'>=':left>=right}[node.op];const a=number(left),b=number(right);if(node.op==='/'&&b===0)fail('#DIV/0!');return {'+':()=>a+b,'-':()=>a-b,'*':()=>a*b,'/':()=>a/b,'^':()=>a**b}[node.op]();}
  if(node.type==='call'){
   if(node.name==='IF'){if(node.args.length!==3)fail('#ARGS!');return calc(node.args[calc(node.args[0])?1:2]);}
   if(!['SUM','AVERAGE','MIN','MAX','COUNT','ROUND','ABS'].includes(node.name))fail('#NAME?');
   const values=node.args.map(calc).flat();const numbers=values.filter(value=>typeof value==='number');
   if(node.name==='SUM')return numbers.reduce((a,b)=>a+b,0);
   if(node.name==='COUNT')return numbers.length;
   if(node.name==='AVERAGE'){if(!numbers.length)fail('#DIV/0!');return numbers.reduce((a,b)=>a+b,0)/numbers.length;}
   if(node.name==='MIN')return numbers.length?Math.min(...numbers):0;
   if(node.name==='MAX')return numbers.length?Math.max(...numbers):0;
   if(node.name==='ABS'){if(values.length!==1)fail('#ARGS!');return Math.abs(number(values[0]));}
   if(node.name==='ROUND'){if(values.length!==2)fail('#ARGS!');const places=number(values[1]);if(!Number.isInteger(places)||Math.abs(places)>10)fail('#ARGS!');const value=number(values[0]),shift=(v,n)=>{const [base,exponent='0']=String(v).split('e');return Number(base+'e'+(Number(exponent)+n));};return Math.sign(value)*shift(Math.round(shift(Math.abs(value),places)),-places);}
  }
  fail('#FORMULA!');
 }
 for(let row=0;row<sheet.rows;row++)for(let col=0;col<sheet.cols;col++){const key=cellKey(row,col);try{result[key]=visit(key);}catch(error){result[key]=error instanceof FormulaError?error.message:'#FORMULA!';}}
 return result;
}
export function parseWorkbook(text){
 if(typeof text!=='string'||text.length>1000000)throw new Error('File too large');const value=JSON.parse(text);
 if(value?.schema!==1||typeof value.title!=='string'||value.title.length>120||!Array.isArray(value.sheets)||!value.sheets.length||value.sheets.length>MAX_SHEETS)throw new Error('Invalid workbook');
 const ids=new Set(),names=new Set();const sheets=value.sheets.map(sheet=>{
  if(typeof sheet.id!=='string'||!sheet.id||sheet.id.length>128||ids.has(sheet.id)||typeof sheet.name!=='string'||!sheet.name.trim()||sheet.name.length>31||names.has(sheet.name)||!Number.isInteger(sheet.rows)||sheet.rows<1||sheet.rows>MAX_ROWS||!Number.isInteger(sheet.cols)||sheet.cols<1||sheet.cols>MAX_COLS||typeof sheet.freeze!=='boolean'||!sheet.cells||typeof sheet.cells!=='object'||Array.isArray(sheet.cells))throw new Error('Invalid sheet');ids.add(sheet.id);names.add(sheet.name);
  const cells={};for(const [key,cell] of Object.entries(sheet.cells)){
   const match=/^([A-Z])([1-9]\d{0,2})$/.exec(key);if(!match||Number(match[2])>sheet.rows||match[1].charCodeAt(0)-65>=sheet.cols||!cell||typeof cell.value!=='string'||cell.value.length>2000||typeof cell.bold!=='boolean'||typeof cell.italic!=='boolean'||!['left','center','right'].includes(cell.align)||!['general','number','currency','percent'].includes(cell.format)||!['none','muted'].includes(cell.fill))throw new Error('Invalid cell');cells[key]={value:cell.value,bold:cell.bold,italic:cell.italic,align:cell.align,format:cell.format,fill:cell.fill};
  }return {id:sheet.id,name:sheet.name,rows:sheet.rows,cols:sheet.cols,freeze:sheet.freeze,cells};
 });return {schema:1,title:value.title,sheets};
}
export function csvText(matrix,delimiter=';'){if(![';',',','\t'].includes(delimiter))throw new Error('Invalid delimiter');return '\uFEFF'+matrix.map(row=>row.map(value=>{let text=String(value);if(/^[=+\-@\t\r]/.test(text)&&!(typeof value==='number'&&Number.isFinite(value)))text="'"+text;return '"'+text.replace(/"/g,'""')+'"';}).join(delimiter)).join('\r\n');}
export function parseDelimited(text){
 if(typeof text!=='string'||text.length>1000000)throw new Error('Paste too large');const input=text.replace(/^\uFEFF/,'');const counts={'\t':0,';':0,',':0};let inQuote=false;for(let i=0;i<input.length;i++){const char=input[i];if(char==='"'){if(inQuote&&input[i+1]==='"'){i++;continue;}inQuote=!inQuote;}else if(!inQuote){if(char==='\r'||char==='\n')break;if(char in counts)counts[char]++;}}const delimiter=Object.keys(counts).sort((a,b)=>counts[b]-counts[a])[0];const rows=[];let row=[],cell='',quoted=false;
 for(let i=0;i<input.length;i++){const char=input[i];if(char==='"'){if(quoted&&input[i+1]==='"'){cell+='"';i++;}else if(!cell||quoted)quoted=!quoted;else cell+=char;}else if(char===delimiter&&!quoted){row.push(cell);cell='';}else if((char==='\n'||char==='\r')&&!quoted){if(char==='\r'&&input[i+1]==='\n')i++;row.push(cell);rows.push(row);row=[];cell='';}else cell+=char;if(cell.length>2000||row.length>MAX_COLS||rows.length>MAX_ROWS)throw new Error('Paste limits');}
 if(quoted)throw new Error('Unclosed quote');if(cell||row.length){row.push(cell);rows.push(row);}if(rows.length>MAX_ROWS||rows.some(row=>row.length>MAX_COLS))throw new Error('Paste limits');return rows;
}
export function sortSheet(sheet,col,descending=false){
 if(!Number.isInteger(col)||col<0||col>=sheet.cols)throw new Error('Invalid column');
 // Sorting formulas requires rewriting references, deliberately blocked rather than corrupting them.
 if(Object.values(sheet.cells).some(cell=>cell.value.startsWith('=')))throw new Error('Sort formulas unsupported');
 const rows=Array.from({length:sheet.rows-1},(_,i)=>i+1);rows.sort((a,b)=>{const left=sheet.cells[cellKey(a,col)]?.value||'',right=sheet.cells[cellKey(b,col)]?.value||'';if(!left||!right)return !left&&!right?0:!left?1:-1;const numeric=/^-?\d+(?:[.,]\d+)?$/;const result=numeric.test(left)&&numeric.test(right)?Number(left.replace(',','.'))-Number(right.replace(',','.')):left.localeCompare(right,undefined,{numeric:true});return descending?-result:result;});
 const cells={};for(let col=0;col<sheet.cols;col++){const header=sheet.cells[cellKey(0,col)];if(header)cells[cellKey(0,col)]={...header};rows.forEach((old,index)=>{const cell=sheet.cells[cellKey(old,col)];if(cell)cells[cellKey(index+1,col)]={...cell};});}return {...sheet,cells};
}
