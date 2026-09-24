import { lazy, Suspense } from 'react';
import { ConfigProvider, Layout, Spin } from 'antd';

import AppSidebar from '@/layouts/AppSidebar';
import { useTheme } from '@/hooks/useTheme';
// NOTE: neither the Tailwind layer (pasarguard.css) nor the phone layer
// (pg-admin-mobile.css) is imported here, deliberately.
//
// Every component this page renders — Button, Card, Dialog, Switch, Badge,
// Separator, Skeleton — is a Tailwind/shadcn component, so this page needs both
// layers. It just cannot get them from here.
//
// Importing them in this file produced `/* empty css */` in the built chunk. The
// bundler hoists a stylesheet that several lazy routes import into a shared CSS
// chunk and attaches that chunk to only some of the importers; this page, which
// imports them directly and has no other pg-ui page beside it, was left out. It
// rendered as unstyled HTML, with the Radix Dialog (no positioning of its own)
// drawn off-screen — so the page looked like it ignored every click.
//
// Both layers are therefore loaded once at the entry point, in main.tsx. Nothing
// on this page should import them again.

/**
 * Standalone wrapper for the account's own API tokens.
 *
 * It is a page of its own rather than a tab on the Admins screen on purpose: the
 * Admins screen needs `admins.read`, which is exactly the permission an operator
 * does not have — so as a tab it was unreachable for the admins who need a token
 * most. This route is gated on the account's API-access flag instead.
 */

const MyApiTokens = lazy(() => import('@/pg-ui/features/admins/components/my-api-tokens'));

export default function MyApiPage() {
  const { antdThemeConfig } = useTheme();

  return (
    <ConfigProvider theme={antdThemeConfig}>
      <Layout style={{ minHeight: '100vh' }}>
        <AppSidebar />
        <Layout className="pg-admin-page bg-background text-foreground" style={{ minWidth: 0 }}>
          <Suspense
            fallback={
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'center',
                  alignItems: 'center',
                  minHeight: '60vh',
                }}
              >
                <Spin size="large" />
              </div>
            }
          >
            <div className="mx-auto w-full max-w-3xl px-4 py-6">
              <MyApiTokens />
            </div>
          </Suspense>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
