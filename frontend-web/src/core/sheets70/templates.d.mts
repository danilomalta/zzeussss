import type { IWorkbookData } from '@univerjs/presets';
export function templateBook(
  base: IWorkbookData,
  kind: string,
  label: (key: string) => string
): IWorkbookData;
