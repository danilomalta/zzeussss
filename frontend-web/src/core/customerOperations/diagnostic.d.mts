export type Diagnostic={version:1;build:string;schema_version:number;observed_unix:number;backup_unix:number;backup_interval_seconds:number;vault:'available'|'unavailable';key_exported:boolean;tls:'loopback'|'direct_tls'|'proxy_declared'|'invalid'};
export function parseDiagnostic(text:string,now?:number):Readonly<Diagnostic>;
export function diagnosticWarnings(report:Diagnostic|null,now?:number):string[];
