export function applyStructure69(sheet,range,operation,size){
 if(!sheet||!range)throw new Error('Selection required');
 const row=range.getRow(),col=range.getColumn(),rows=range.getHeight(),cols=range.getWidth();
 if(![row,col,rows,cols].every(Number.isInteger)||row<0||col<0||rows<1||cols<1||row+rows>sheet.getMaxRows()||col+cols>sheet.getMaxColumns())throw new Error('Invalid selection');
 switch(operation){
 case 'insertRows':if(sheet.getMaxRows()+rows>10000)throw new Error('Row limit');return sheet.insertRowsBefore(row,rows);
 case 'deleteRows':if(rows>=sheet.getMaxRows())throw new Error('Last row');return sheet.deleteRows(row,rows);
 case 'hideRows':if(rows>=sheet.getMaxRows())throw new Error('All rows');return sheet.hideRows(row,rows);
 case 'showRows':return sheet.showRows(row,rows);
 case 'height':if(!Number.isInteger(size)||size<16||size>400)throw new Error('Height');return sheet.setRowHeightsForced(row,rows,size);
 case 'autoHeight':return sheet.setRowAutoHeight(row,rows);
 case 'insertColumns':if(sheet.getMaxColumns()+cols>200)throw new Error('Column limit');return sheet.insertColumnsBefore(col,cols);
 case 'deleteColumns':if(cols>=sheet.getMaxColumns())throw new Error('Last column');return sheet.deleteColumns(col,cols);
 case 'hideColumns':if(cols>=sheet.getMaxColumns())throw new Error('All columns');return sheet.hideColumns(col,cols);
 case 'showColumns':return sheet.showColumns(col,cols);
 case 'width':if(!Number.isInteger(size)||size<30||size>800)throw new Error('Width');return sheet.setColumnWidths(col,cols,size);
 case 'showAll':sheet.showRows(0,sheet.getMaxRows());return sheet.showColumns(0,sheet.getMaxColumns());
 default:throw new Error('Unknown operation');
 }
}
export function previewMatrix69(snapshot,id,selection){
 const sheet=snapshot.sheets[id];if(!sheet)throw new Error('Sheet');let endRow=-1,endColumn=-1;
 if(selection){if(![selection.startRow,selection.endRow,selection.startColumn,selection.endColumn].every(Number.isInteger))throw new Error('Range');endRow=selection.endRow;endColumn=selection.endColumn;}else{for(const [r,row] of Object.entries(sheet.cellData))for(const [c,cell] of Object.entries(row))if(cell.v!=null||cell.f){endRow=Math.max(endRow,Number(r));endColumn=Math.max(endColumn,Number(c));}}
 const startRow=selection?.startRow||0,startColumn=selection?.startColumn||0;
 if(startRow<0||startColumn<0||endRow<startRow||endColumn<startColumn||endRow>=sheet.rowCount||endColumn>=sheet.columnCount||endRow-startRow>=200||endColumn-startColumn>=20)throw new Error('Print range limit');
 return Array.from({length:endRow-startRow+1},(_,r)=>Array.from({length:endColumn-startColumn+1},(_,c)=>sheet.cellData[r+startRow]?.[c+startColumn]?.v??''));
}
