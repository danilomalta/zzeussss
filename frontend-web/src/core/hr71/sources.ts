import { staff } from './staff';
import { attendance } from './attendance';
import { schedules } from './schedules';
import { leave } from './leave';
import { documents } from './documents';
import { payroll } from './payroll';
import { finance } from './finance';
import { alerts } from './alerts';
import type { Adapter, Sources } from './contracts';
export const adapters: { [K in keyof Sources]: Adapter<K> } = {
  staff,
  attendance,
  schedules,
  leave,
  documents,
  payroll,
  finance,
  alerts,
};
