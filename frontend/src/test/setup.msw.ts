import { afterAll, afterEach, beforeAll } from 'vitest';

import { server } from './msw/server';

beforeAll(() => {
  server.listen({ onUnhandledRequest: 'bypass' } as unknown as Parameters<typeof server.listen>[0]);
});
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
