import type { ReactNode } from 'react';
import { fireEvent, render, waitFor } from '@testing-library/react';
import { QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import ClientsPage from '@/pages/clients/ClientsPage';
import type { ClientsSummary } from '@/schemas/client';
import { makeTestQueryClient } from '@/test/test-utils';
import { HttpUtil, Msg } from '@/utils';

vi.mock('@/layouts/AppSidebar', () => ({ default: () => null }));
vi.mock('@/components/NeonLandscape', () => ({ default: () => null }));
vi.mock('@/hooks/useTheme', () => ({
  useTheme: () => ({ isDark: false, isUltra: false, antdThemeConfig: {} }),
}));
vi.mock('@/hooks/useMediaQuery', () => ({ useMediaQuery: () => ({ isMobile: true }) }));
vi.mock('@/hooks/useDatepicker', () => ({ useDatepicker: () => ({ datepicker: 'gregorian' }) }));
vi.mock('@/hooks/useWebSocket', () => ({ useWebSocket: () => {} }));
vi.mock('@/api/queries/useNodesQuery', () => ({ useNodesQuery: () => ({ nodes: [] }) }));
vi.mock('@/api/queries/useHostsQuery', () => ({
  useHostsQuery: () => ({ hosts: [], fetched: true, fetchError: '', refetch: async () => {} }),
}));

const owner: ClientsSummary = {
  total: 205,
  onlineCount: 201,
  depletedCount: 30,
  expiringCount: 40,
  deactiveCount: 35,
  active: 100,
  online: ['owner@example.test'],
  depleted: ['ended@example.test'],
  expiring: ['soon@example.test'],
  deactive: ['disabled@example.test'],
};
const alice: ClientsSummary = {
  total: 3,
  onlineCount: 1,
  depletedCount: 1,
  expiringCount: 0,
  deactiveCount: 1,
  active: 1,
  online: ['alice@example.test'],
  depleted: ['alice-ended@example.test'],
  expiring: [],
  deactive: ['alice-disabled@example.test'],
};
const bob: ClientsSummary = {
  total: 1,
  onlineCount: 0,
  depletedCount: 0,
  expiringCount: 0,
  deactiveCount: 0,
  active: 1,
  online: [],
  depleted: [],
  expiring: [],
  deactive: [],
};

beforeEach(() => localStorage.clear());
afterEach(() => vi.restoreAllMocks());

function renderPage(summary: ClientsSummary, email: string) {
  const requests: URLSearchParams[] = [];
  vi.spyOn(HttpUtil, 'get').mockImplementation(async (url: string) => {
    if (url.includes('/clients/list/paged')) {
      const params = new URLSearchParams(url.split('?')[1]);
      requests.push(params);
      return new Msg(true, '', {
        items: [{ email, enable: true, traffic: { up: 0, down: 0 } }],
        total: summary.total,
        filtered: params.has('filter') ? 1 : summary.total,
        page: Number(params.get('page')),
        pageSize: Number(params.get('pageSize')),
        groups: [],
        summary,
      });
    }
    if (url.includes('/inbounds/options')) return new Msg(true, '', []);
    return new Msg(true, '', null);
  });
  vi.spyOn(HttpUtil, 'post').mockImplementation(async (url: string) => {
    if (url.includes('/setting/defaultSettings')) return new Msg(true, '', { pageSize: 25 });
    if (url.includes('/clients/onlines')) return new Msg(true, '', summary.online);
    return new Msg(true, '', null);
  });
  const queryClient = makeTestQueryClient();
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{children}</MemoryRouter>
    </QueryClientProvider>
  );
  return { ...render(<ClientsPage />, { wrapper }), requests };
}

function displayedCounts(container: HTMLElement, selector: string): number[] {
  return Array.from(container.querySelectorAll(selector), (element) => Number(element.textContent));
}

describe('Clients page scoped summary', () => {
  it.each([
    ['owner', owner, 'owner@example.test'],
    ['alice', alice, 'alice@example.test'],
    ['bob', bob, 'bob@example.test'],
  ])(
    'renders %s server totals in both the header and all six cards',
    async (_role, summary, email) => {
      const { container } = renderPage(summary, email);
      await waitFor(() => expect(container.querySelectorAll('.nc-stat-value')).toHaveLength(6));
      expect(displayedCounts(container, '.nc-hero-metric-value')).toEqual([
        summary.total,
        summary.onlineCount,
      ]);
      expect(displayedCounts(container, '.nc-stat-value')).toEqual([
        summary.total,
        summary.onlineCount,
        summary.depletedCount,
        summary.expiringCount,
        summary.deactiveCount,
        summary.active,
      ]);
      expect(container.textContent).toContain(email);
    },
  );

  it('keeps the same summary while paging and filtering the owner list', async () => {
    const { container, requests } = renderPage(owner, 'owner@example.test');
    await waitFor(() => expect(container.querySelectorAll('.nc-stat-value')).toHaveLength(6));
    const values = [
      owner.total,
      owner.onlineCount,
      owner.depletedCount,
      owner.expiringCount,
      owner.deactiveCount,
      owner.active,
    ];
    const next = container.querySelector('.ant-pagination-next button');
    expect(next).not.toBeNull();
    fireEvent.click(next!);
    await waitFor(() => expect(requests.some((p) => p.get('page') === '2')).toBe(true));
    expect(displayedCounts(container, '.nc-stat-value')).toEqual(values);

    const endedCard = container.querySelectorAll('.nc-stat')[2];
    fireEvent.click(endedCard);
    await waitFor(() =>
      expect(requests.some((p) => p.get('page') === '1' && p.get('filter') === 'depleted')).toBe(
        true,
      ),
    );
    expect(displayedCounts(container, '.nc-hero-metric-value')).toEqual([
      owner.total,
      owner.onlineCount,
    ]);
    expect(displayedCounts(container, '.nc-stat-value')).toEqual(values);
  });
});
