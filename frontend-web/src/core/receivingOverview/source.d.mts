import type { LocalSession, LocalCapabilities } from '../local/localClient.mjs';
import type { ReceivingOverview, ReceivingBoard } from './contracts';
import type { ReceivingFilters } from './model.mjs';
export interface ReceivingOverviewSource {
    scope: string;
    assertCurrent(): void;
    dispose(): void;
    overview(id: string, offset?: number, signal?: AbortSignal): Promise<ReceivingOverview>;
    board(filters: ReceivingFilters, offset?: number, signal?: AbortSignal): Promise<ReceivingBoard>;
}
export function createReceivingOverview(options: {
    token: string;
    session: LocalSession;
    capabilities: LocalCapabilities;
    isCurrent: () => boolean;
    fetcher?: typeof fetch;
}): ReceivingOverviewSource;
