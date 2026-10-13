export const hrSections=['overview','recruitment','lifecycle','attendance','schedules','leave','payroll','benefits','compliance','performance','reports','portal','security'];
export const hrEditableSections=hrSections.filter(id=>!['overview','reports','security'].includes(id));
export function validateHRDraft(value){
 if(!value||!hrEditableSections.includes(value.section)||typeof value.subject!=='string'||!value.subject.trim()||value.subject.length>120||typeof value.person!=='string'||value.person.length>120||typeof value.notes!=='string'||value.notes.length>2000||typeof value.date!=='string'||typeof value.endDate!=='string'||typeof value.email!=='string'||value.email.length>254||!['planned','review','ready'].includes(value.stage)||!['amount','startTime','endTime'].every(key=>typeof value[key]==='string'))return false;
 for(const key of ['date','endDate'])if(value[key]&&(!/^\d{4}-\d{2}-\d{2}$/.test(value[key])||!Number.isFinite(Date.parse(value[key]+'T12:00:00Z'))||new Date(value[key]+'T12:00:00Z').toISOString().slice(0,10)!==value[key]))return false;
 if(value.date&&value.endDate&&value.endDate<value.date)return false;
 if(value.email&&!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value.email))return false;
 if(value.amount&&!/^\d{1,9}([.,]\d{1,2})?$/.test(value.amount))return false;
 for(const key of ['startTime','endTime'])if(value[key]&&!/^([01]\d|2[0-3]):[0-5]\d$/.test(value[key]))return false;
 return true;
}
export function createHRDraft(input,id){
 if(!validateHRDraft(input)||typeof id!=='string'||!id)throw new Error('Invalid HR draft');
 return {id,section:input.section,subject:input.subject.trim(),person:input.person.trim(),notes:input.notes.trim(),date:input.date,endDate:input.endDate,email:input.email,stage:input.stage,amount:input.amount,startTime:input.startTime,endTime:input.endTime};
}
