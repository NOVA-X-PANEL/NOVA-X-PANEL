import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import MyApiTokens from '@/pg-ui/features/admins/components/my-api-tokens';
import { HttpUtil } from '@/utils';

/*
 * Behaviour of the self-service token list.
 *
 * The page shipped broken in a way that no assertion here would have caught on
 * its own, because the defect was a 404 hidden behind `catch { return [] }` — the
 * list rendered an empty state and looked like a working page with no tokens.
 *
 * So these tests assert the *observable* difference between the three outcomes:
 * rows render, an empty result says so, and a failure names the reason. The last
 * one is the regression guard: a failure must never look like an empty list.
 */

function makeClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

function renderPage() {
  return render(
    <QueryClientProvider client={makeClient()}>
      <MyApiTokens />
    </QueryClientProvider>,
  );
}

function envelope(obj: unknown, success = true, msg = '') {
  return { success, msg, obj };
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe('My API token list', () => {
  it('renders the tokens the server returns', async () => {
    const get = vi.spyOn(HttpUtil, 'get').mockResolvedValue(
      envelope([
        {
          id: 1,
          name: 'ci-deploy',
          enabled: true,
          scope: 'admin',
          expiresAt: 0,
          createdAt: 1782485394,
        },
        {
          id: 2,
          name: 'monitoring',
          enabled: false,
          scope: 'admin',
          expiresAt: 0,
          createdAt: 1782485394,
        },
      ]),
    );

    renderPage();

    expect(await screen.findByText('ci-deploy')).toBeTruthy();
    expect(screen.getByText('monitoring')).toBeTruthy();

    // The list is a GET. Calling it with the wrong method is what broke the page:
    // the route is `g.GET("/apiTokens")`, so a POST returned 404 and the old
    // catch turned that into a permanently empty list.
    expect(get).toHaveBeenCalledTimes(1);
    expect(get.mock.calls[0][0]).toBe('/panel/api/admins/apiTokens');
  });

  it('shows an empty state when the account genuinely has no tokens', async () => {
    vi.spyOn(HttpUtil, 'get').mockResolvedValue(envelope([]));

    renderPage();

    expect(await screen.findByText('No tokens yet.')).toBeTruthy();
  });

  it('names the reason when the load fails instead of showing an empty list', async () => {
    vi.spyOn(HttpUtil, 'get').mockResolvedValue(
      envelope(null, false, 'API access is not enabled for this account'),
    );

    renderPage();

    // The reason the server gave must reach the screen...
    expect(await screen.findByText('API access is not enabled for this account')).toBeTruthy();

    // ...and it must not be presented as "you have no tokens". Conflating the two
    // is what made the original bug take a bug report to find.
    expect(screen.queryByText('No tokens yet.')).toBeNull();
  });

  it('does not create a token when the list fails to load', async () => {
    vi.spyOn(HttpUtil, 'get').mockResolvedValue(envelope(null, false, 'nope'));
    const post = vi.spyOn(HttpUtil, 'post').mockResolvedValue(envelope(null));

    renderPage();
    await screen.findByText('nope');

    // Rendering the list must not have a side effect on the account's tokens.
    expect(post).not.toHaveBeenCalled();
  });

  it('shows a created token even when the refresh afterwards fails', async () => {
    // This is the reported failure, stated as a test: create succeeds, the refresh
    // that follows does not, and the row never appeared — so the page looked as
    // though "Create" had done nothing at all.
    //
    // The create response already carries every field the list renders, so the row
    // is written to the cache directly instead of waiting for the refetch.
    const get = vi.spyOn(HttpUtil, 'get').mockResolvedValue(envelope([]));
    vi.spyOn(HttpUtil, 'post').mockResolvedValue(
      envelope({
        id: 71,
        name: 'survives-a-failed-refresh',
        token: 'PLAINTEXT-SHOULD-NOT-REACH-THE-LIST',
        enabled: true,
        scope: 'admin',
        expiresAt: 0,
        createdAt: 1782485394,
      }),
    );

    renderPage();
    await screen.findByText('No tokens yet.');

    // Every later list read fails, so only the optimistic write can supply the row.
    get.mockResolvedValue(envelope(null, false, 'the panel could not be reached'));

    fireEvent.click(screen.getByRole('button', { name: /Create token/i }));
    fireEvent.change(await screen.findByLabelText('Name'), {
      target: { value: 'survives-a-failed-refresh' },
    });
    fireEvent.click(screen.getByRole('button', { name: /^Create$/i }));

    // The row is visible...
    expect(await screen.findByText('survives-a-failed-refresh')).toBeTruthy();
    // ...the empty state is gone...
    expect(screen.queryByText('No tokens yet.')).toBeNull();
    // ...and the plaintext was shown in the one-time reveal, not written into the
    // list, which never renders it.
    expect(await screen.findByText('PLAINTEXT-SHOULD-NOT-REACH-THE-LIST')).toBeTruthy();
  });
});
