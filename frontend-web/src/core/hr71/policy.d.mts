import type { LocalCapabilities } from '../local/localClient.mjs';
import type { Person, SourceState } from './contracts';
export interface Area {
  id: string;
  icon: string;
  major?: boolean;
  route?: string;
  sensitive?: boolean;
}
export const areas: Area[];
export function canView(capabilities: LocalCapabilities | null): boolean;
export function sourceAccess(name: string, capabilities: LocalCapabilities | null): SourceState;
export function visibleAreas(capabilities: LocalCapabilities | null): Area[];
export function areaState(area: Area, capabilities: LocalCapabilities | null): SourceState;
export function initials(name: string): string;
export function filterPeople(
  people: Person[],
  filters: {
    query?: string;
    department?: string;
    jobTitle?: string;
    status?: string;
    ids?: string[];
    sort?: string;
  }
): Person[];
export function csv(rows: unknown[][]): string;
export function currency(cents: number, locale: string): string;
