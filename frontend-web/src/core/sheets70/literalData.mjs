// External labels are literal data, never formulas or locale-parsed numbers.
// Clear formula/rich-text fields when refreshing a previously populated range.
export function literalSheetMatrix(matrix){
 if(!Array.isArray(matrix)||!matrix.length||matrix.length>501||!Array.isArray(matrix[0])||!matrix[0].length||matrix[0].length>200)throw Error('sheetData');
 const width=matrix[0].length;
 return matrix.map(row=>{if(!Array.isArray(row)||row.length!==width)throw Error('sheetData');return row.map(value=>{
  const type=typeof value;if(!['string','number','boolean'].includes(type)||type==='number'&&(!Number.isFinite(value)||Math.abs(value)>Number.MAX_SAFE_INTEGER))throw Error('sheetData');
  return {v:value,t:type==='string'?1:type==='number'?2:3,f:null,p:null};
 });});
}
