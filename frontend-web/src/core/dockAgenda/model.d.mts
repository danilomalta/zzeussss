import type { Dock, Appointment, Result, DockPage, AppointmentPage, OccupancyPage, EventPage, Detail, DockInput, BookInput, ActionInput } from './contracts';
export type DockBody = DockInput | BookInput | ActionInput;
export interface DockCommand {
    version: 1;
    scope: string;
    kind: 'dock' | 'reserve' | 'action';
    entity_id: string;
    before: Dock | Appointment | null;
    body: DockBody;
    uncertain: boolean;
}
export const dockStatuses: string[];
export const dockActions: Record<string, string[]>;
export function dockFail(): never;
export function dockID(v: unknown): boolean;
export function dockText(v: unknown, max: number): boolean;
export function dockEqual(a: unknown, b: unknown): boolean;
export function dockSnapshot(v: unknown): string;
export function dockTime(v: unknown, whole?: boolean): bigint;
export function dockUTC(v: string): string;
export function dockLocalInput(v: string): string;
export function dockWindow(from: string, to: string, days?: boolean): {
    from: string;
    to: string;
};
export function dockResource(v: unknown): Dock;
export function dockAppointment(v: unknown): Appointment;
export function dockResult(v: unknown): Result;
export function dockValidate(name: string, v: unknown): unknown;
export function dockPage(name: 'DockPage', v: unknown, offset: number, limit?: number): DockPage;
export function dockPage(name: 'AppointmentPage', v: unknown, offset: number, limit?: number): AppointmentPage;
export function dockPage(name: 'OccupancyPage', v: unknown, offset: number, limit?: number): OccupancyPage;
export function dockPage(name: 'EventPage', v: unknown, offset: number, limit?: number): EventPage;
export function dockInput(kind: string, v: unknown): DockBody;
export function dockCommand(v: unknown, scope: string): DockCommand;
export function dockReceipt(v: unknown, command: DockCommand, actor: string): Result;
export function dockDetail(v: unknown, id: string): Detail;
export function dockHistory(v: unknown, kind: string, id: string, offset: number, limit?: number): EventPage;
export function dockCSV(page: AppointmentPage, window: {
    from: string;
    to: string;
}): string;
export function dockCompareID(a: string, b: string): number;
