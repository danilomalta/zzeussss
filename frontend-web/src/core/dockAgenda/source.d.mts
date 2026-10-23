import type { LocalSession, LocalCapabilities } from '../local/localClient.mjs';
import type { Dock, Appointment, Result, DockPage, AppointmentPage, OccupancyPage, Detail, EventPage } from './contracts';
import type { DockCommand, DockBody } from './model.mjs';
export type DockOutcome = {
    state: 'confirmed' | 'missing' | 'discarded';
    command: DockCommand;
    receipt?: Result;
};
export interface DockSource {
    scope: string;
    canWrite: boolean;
    canManage: boolean;
    canLock: boolean;
    assertCurrent(): void;
    pending(): DockCommand | null;
    docks(offset?: number, signal?: AbortSignal): Promise<DockPage>;
    occupancy(offset?: number, signal?: AbortSignal): Promise<OccupancyPage>;
    appointments(filters: {
        from: string;
        to: string;
        dock_id?: string;
        status?: string;
    }, offset?: number, signal?: AbortSignal): Promise<AppointmentPage>;
    detail(id: string, signal?: AbortSignal): Promise<Detail>;
    history(kind: 'dock' | 'appointment', id: string, offset?: number, signal?: AbortSignal): Promise<EventPage>;
    prepare(kind: 'dock' | 'reserve' | 'action', id: string, before: Dock | Appointment | null, body: DockBody): DockCommand;
    submit(command: DockCommand): Promise<DockOutcome | null>;
    recover(send?: boolean, discard?: boolean): Promise<DockOutcome | null>;
}
export function createDockAgenda(options: {
    token: string;
    session: LocalSession;
    capabilities: LocalCapabilities;
    isCurrent: () => boolean;
    storage: Storage;
    locks?: LockManager;
    fetcher?: typeof fetch;
}): DockSource;
