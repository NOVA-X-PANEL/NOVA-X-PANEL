import { lazy, Suspense } from 'react';
import { ConfigProvider, Layout, Spin } from 'antd';

import AppSidebar from '@/layouts/AppSidebar';
import { useTheme } from '@/hooks/useTheme';
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
