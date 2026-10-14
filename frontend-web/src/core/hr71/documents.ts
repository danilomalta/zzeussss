import { missing } from './base';
import type { Adapter } from './contracts';
export const documents: Adapter<'documents'> = (context) => missing('documents', context);
