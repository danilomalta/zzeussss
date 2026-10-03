export const cashDenominations = [20000,10000,5000,2000,1000,500,200,100,50,25,10,5,1];
export function cashCount(counts) {
 let total=0n;
 for (const [index,value] of counts.entries()) {
  if (index>=cashDenominations.length || !/^(0|[1-9][0-9]{0,8})$/.test(value || '0')) throw new Error('Informe quantidades inteiras de notas e moedas.');
  total+=BigInt(value || '0')*BigInt(cashDenominations[index]);
 }
 if(total>BigInt(Number.MAX_SAFE_INTEGER)) throw new Error('Contagem fora do intervalo suportado.');
 return Number(total);
}
