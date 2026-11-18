import type { ReactNode } from 'react';
import { act, renderHook, waitFor } from '@testing-library/react';
import { QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { keys } from '@/api/queryKeys';
import { useClients } from '@/hooks/useClients';
import type { ClientsSummary } from '@/schemas/client';
import { makeTestQueryClient } from '@/test/test-utils';
import { HttpUtil, Msg } from '@/utils';

const ownerSummary: ClientsSummary = {
  total: 205,
  active: 100,
  onlineCount: 201,
  depletedCount: 30,
  expiringCount: 40,
  deactiveCount: 35,
  // The server caps these popover arrays; they are not the counts.
  online: ['owner-online@example.test'],
  depleted: ['owner-ended@example.test'],
  expiring: ['owner-soon@example.test'],
  deactive: ['owner-disabled@example.test'],
};

const aliceSummary: ClientsSummary = {
  total: 3,
  active: 1,
  onlineCount: 1,
  depletedCount: 1,
  expiringCount: 0,
  deactiveCount: 1,
  online: ['alice@example.test'],
  depleted: ['alice-ended@example.test'],
  expiring: [],
  deactive: ['alice-disabled@example.test'],
};

const bobSummary: ClientsSummary = {
  total: 1,
  active: 1,
  onlineCount: 0,
  depletedCount: 0,
  expiringCount: 0,
  deactiveCount: 0,
  online: [],
  depleted: [],
  expiring: [],
  deactive: [],
};

afterEach(() => vi.restoreAllMocks());

function setup(summary: ClientsSummary, email: string, online: string[]) {
  let scopedOnline = online;
  const requests: URLSearchParams[] = [];
  vi.spyOn(HttpUtil, 'get').mockImplementation(async (url: string) => {
    if (url.includes('/clients/list/paged')) {
      const params = new URLSearchParams(url.split('?')[1]);
      requests.push(params);
      return new Msg(true, '', {
        items: [{ email, enable: true, traffic: { up: 1, down: 2 } }],
        total: summary.total,
        filtered: params.has('filter') || params.has('search') ? 1 : summary.total,
        page: Number(params.get('page')),
        pageSize: Number(params.get('pageSize')),
        groups: [],
        summary,
      });
    }
    if (url.includes('/inbounds/options')) return new Msg(true, '', []);
    return new Msg(true, '', null);
  });
  const post = vi.spyOn(HttpUtil, 'post').mockImplementation(async (url: string) => {
    if (url.includes('/setting/defaultSettings')) return new Msg(true, '', { pageSize: 25 });
    if (url.includes('/clients/onlines')) return new Msg(true, '', [...scopedOnline]);
    return new Msg(true, '', null);
  });
  const queryClient = makeTestQueryClient();
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  const hook = renderHook(() => useClients(), { wrapper });
  return { ...hook, queryClient, requests, post, setOnline: (v: string[]) => (scopedOnline = v) };
}

describe('Clients scope from the server', () => {
  it.each([
    ['owner', ownerSummary, 'owner@example.test'],
    ['alice', aliceSummary, 'alice@example.test'],
    ['bob', bobSummary, 'bob@example.test'],
  ])(
    'keeps the %s summary independent of page, filter, and capped email arrays',
    async (_role, summary, email) => {
      const { result, requests } = setup(summary, email, summary.online);
      act(() => result.current.setQuery({ page: 1, pageSize: 25 }));
      await waitFor(() => expect(result.current.summary).toEqual(summary));
      expect(result.current.total).toBe(summary.total);
      expect(result.current.filtered).toBe(summary.total);
      expect(result.current.clients.map((c) => c.email)).toEqual([email]);

      act(() =>
        result.current.setQuery({ page: 2, pageSize: 10, filter: 'depleted', search: 'alice' }),
      );
      await waitFor(() => expect(requests).toHaveLength(2));
      await waitFor(() => expect(result.current.transitioning).toBe(false));
      expect(requests[1].get('page')).toBe('2');
      expect(requests[1].get('pageSize')).toBe('10');
      expect(requests[1].get('filter')).toBe('depleted');
      expect(requests[1].get('search')).toBe('alice');
      expect(result.current.filtered).toBe(1);
      expect(result.current.total).toBe(summary.total);
      expect(result.current.summary).toEqual(summary);
      expect(result.current.clients.map((c) => c.email)).toEqual([email]);
    },
  );

  it('refreshes online data through scoped REST instead of caching a panel-wide websocket list', async () => {
    const { result, queryClient, post, setOnline } = setup(aliceSummary, 'alice@example.test', [
      'alice@example.test',
    ]);
    act(() => result.current.setQuery({ page: 1, pageSize: 25 }));
    await waitFor(() => expect(result.current.onlines).toEqual(['alice@example.test']));
    await waitFor(() => expect(result.current.summary).toEqual(aliceSummary));

    // Alice went offline while a foreign client remained online. The broadcast
    // may be panel-wide, but the scoped endpoint is authoritative for the cache.
    setOnline([]);
    act(() => {
      result.current.applyTrafficEvent({ onlineClients: ['bob@example.test'] });
    });
    expect(queryClient.getQueryData(keys.clients.onlines())).not.toContain('bob@example.test');
    await waitFor(() => expect(result.current.onlines).toEqual([]));
    expect(
      post.mock.calls.filter(([url]) => String(url).includes('/clients/onlines')),
    ).toHaveLength(2);
    expect(result.current.summary).toEqual(aliceSummary);

    setOnline(['alice@example.test']);
    act(() => {
      result.current.applyTrafficEvent({
        onlineClients: ['alice@example.test', 'bob@example.test'],
      });
      result.current.applyClientStatsEvent({
        clients: [
          { email: 'alice@example.test', up: 10, down: 20 },
          { email: 'bob@example.test', up: 500, down: 500 },
        ],
      });
    });
    await waitFor(() => expect(result.current.onlines).toEqual(['alice@example.test']));
    expect(result.current.clients.map((c) => c.email)).toEqual(['alice@example.test']);
    expect(result.current.clients[0].traffic).toMatchObject({ up: 10, down: 20 });
    expect(result.current.summary).toEqual(aliceSummary);
  });
});
