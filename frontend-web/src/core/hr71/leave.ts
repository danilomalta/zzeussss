import { missing } from './base';
import type { Adapter } from './contracts';
export const leave: Adapter<'leave'> = (context) => missing('leave', context);
