import type { Person, Sources } from './contracts';
const id = (n: number) => '00000000-0000-4000-8000-' + String(n).padStart(12, '0');
const date = (days: number) => {
  const value = new Date();
  value.setDate(value.getDate() + days);
  return value.toISOString().slice(0, 10);
};
export function mockSource<K extends keyof Sources>(name: K, empty: boolean): Sources[K] {
  const people: Person[] = empty
    ? []
    : Array.from({ length: 8 }, (_, index) => ({
        id: id(index + 1),
        name: 'Demo ' + String(index + 1).padStart(2, '0'),
        role: index === 0 ? 'manager' : 'employee',
        status: index === 6 ? 'vacation' : index === 7 ? 'away' : 'access_active',
        jobTitle: index % 2 ? 'job.operations' : 'job.service',
        department: index % 2 ? 'department.operations' : 'department.service',
        nextExpiry: index < 2 ? date(index + 5) : undefined,
      }));
  const metric = (ids: number[], delta?: number) => ({
    count: empty ? 0 : ids.length,
    people: empty ? [] : ids.map(id),
    delta,
  });
  const data: Sources = {
    staff: people,
    attendance: { present: metric([1, 2, 3, 4], 1), absent: metric([5, 6], -1) },
    schedules: { pending: empty ? 0 : 2 },
    leave: { away: metric([7, 8]), pending: metric([3, 5]) },
    documents: { trial: metric([1, 2]), birthdays: metric([4]) },
    payroll: { totalCents: empty ? 0 : 2450000 },
    finance: { costCents: empty ? 0 : 3010000 },
    alerts: empty
      ? []
      : [
          {
            id: 'a1',
            personId: id(1),
            personName: 'Demo 01',
            kind: 'trial',
            severity: 'danger',
            due: date(5),
          },
          {
            id: 'a2',
            personId: id(5),
            personName: 'Demo 05',
            kind: 'missing_point',
            severity: 'warning',
            due: date(0),
          },
          {
            id: 'a3',
            personId: id(3),
            personName: 'Demo 03',
            kind: 'document',
            severity: 'neutral',
            due: date(10),
          },
        ],
  };
  return data[name];
}
