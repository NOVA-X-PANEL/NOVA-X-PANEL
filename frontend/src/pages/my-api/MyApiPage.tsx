import { lazy, Suspense } from 'react';
import { ConfigProvider, Layout, Spin } from 'antd';

import AppSidebar from '@/layouts/AppSidebar';
import { useTheme } from '@/hooks/useTheme';
// The Tailwind layer, and the ONLY place the semantic colour tokens
// (--card, --muted-foreground, --border, --accent, …) are defined.
//
// Every component this page renders — Button, Card, Dialog, Switch, Badge,
// Separator, Skeleton — is a Tailwind/shadcn component, so without this import
// they all fall back to unstyled HTML. The Dialog is the visible casualty: it is
// a Radix portal with no positioning styles, so it renders off-screen and the
// page appears to ignore every click.
//
// The other vendored screens (`_dashboard.admins`, `_dashboard.admin-roles`)
// import it inside their own shell, which is why only this page was broken.
import '@/pg-ui/styles/pasarguard.css';
import '@/styles/pg-admin-mobile.css';

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
