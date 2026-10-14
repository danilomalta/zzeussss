import type { LocalCapabilities, LocalSession } from '../local/localClient.mjs';
export type SourceState =
  'loading' | 'ready' | 'empty' | 'error' | 'forbidden' | 'not_licensed' | 'unavailable';
export interface Source<T> {
  state: SourceState;
  data?: T;
  at?: number;
  mock?: boolean;
}
export interface Context {
  token: string;
  capabilities: LocalCapabilities;
  session: LocalSession;
  signal: AbortSignal;
  mock: boolean;
  empty: boolean;
}
export interface Person {
  id: string;
  name: string;
  role: string;
  status: string;
  jobTitle?: string;
  department?: string;
  nextExpiry?: string;
}
export interface Metric {
  count: number;
  people: string[];
  delta?: number;
}
export interface Attendance {
  present: Metric;
  absent: Metric;
}
export interface Leave {
  away: Metric;
  pending: Metric;
}
export interface Documents {
  trial: Metric;
  birthdays: Metric;
}
export interface Alert {
  id: string;
  personId: string;
  personName: string;
  kind: 'trial' | 'missing_point' | 'document' | 'vacation' | 'exam';
  severity: 'danger' | 'warning' | 'neutral';
  due: string;
}
export interface Sources {
  staff: Person[];
  attendance: Attendance;
  schedules: { pending: number };
  leave: Leave;
  documents: Documents;
  payroll: { totalCents: number };
  finance: { costCents: number };
  alerts: Alert[];
}
export type SourceName = keyof Sources;
export type Adapter<K extends SourceName> = (context: Context) => Promise<Source<Sources[K]>>;
