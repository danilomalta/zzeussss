import { sourceAccess } from './policy.mjs';
import { mockSource } from './mock';
import type { Context, Source, SourceName, Sources } from './contracts';
export async function missing<K extends SourceName>(
  name: K,
  context: Context
): Promise<Source<Sources[K]>> {
  const state = sourceAccess(name, context.capabilities);
  if (state !== 'ready') return { state };
  if (context.signal.aborted) throw new DOMException('Aborted', 'AbortError');
  if (!context.mock) return { state: 'unavailable' };
  return {
    state: context.empty ? 'empty' : 'ready',
    data: mockSource(name, context.empty),
    at: Date.now(),
    mock: true,
  };
}
