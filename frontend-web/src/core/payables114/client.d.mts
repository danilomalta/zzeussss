import type {Agenda114,Window114} from './agenda.mjs';export type AgendaSource114=(query:Window114,signal?:AbortSignal)=>Promise<Agenda114>;
export function createAgenda114(token:string,fetcher?:typeof fetch):AgendaSource114;
