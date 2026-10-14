import { catalog } from './catalog';
import { sales } from './sales';
import { staff } from './staff';
import { purchases } from './purchases';
import { stock } from './stock';
import { finance } from './finance';
import type { Adapter } from './contracts';
export const sources: Record<string, Adapter> = {
  catalog,
  sales,
  staff,
  purchases,
  stock,
  finance,
};
