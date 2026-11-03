export function users112(value){if(typeof value!=='string'||!value.trim()||value.length>32)return null;const n=Number(value);return Number.isSafeInteger(n)&&n>=1&&n<=10000?n:null;}
export function adjustUsers112(value,delta){if(delta!==1&&delta!==-1)throw Error('invalid112');const n=users112(value);return String(n===null?1:Math.max(1,Math.min(10000,n+delta)));}
