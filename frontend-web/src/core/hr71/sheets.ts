import type { LocalCapabilities } from '../local/localClient.mjs';
import { allowedAreas } from '../local/accessModel.mjs';
const bridges = import.meta.glob('../sheets70/sources.ts');
export interface StaffSheetLink {
  available: boolean;
  href: string;
}
export function staffSheetLink(
  capabilities: LocalCapabilities | null,
  mock: boolean
): StaffSheetLink {
  return {
    available:
      !mock && Object.keys(bridges).length > 0 && allowedAreas(capabilities).includes('sheets'),
    href: '/local/sheets?source=staff',
  };
}
