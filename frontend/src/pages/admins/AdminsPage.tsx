import { lazy, Suspense } from 'react';
import { ConfigProvider, Layout, Spin } from 'antd';

import AppSidebar from '@/layouts/AppSidebar';
import { useTheme } from '@/hooks/useTheme';

// The ported pg-ui screens render only their own content — Heimdall's app shell
// supplies the sidebar and theming. NOVA X PANEL's other pages each render that
// shell themselves, so this wrapper does the same for the admin screens and
// hands the pg-ui page a full-width column with no extra padding (its
// PageHeader owns the page padding), keeping it pixel-identical to upstream.

const AdminsPage = lazy(() => import('@/pg-ui/pages/_dashboard.admins'));

export default function AdminsPageWrapper() {
  const { antdThemeConfig } = useTheme();

  return (
    <ConfigProvider theme={antdThemeConfig}>
      <Layout style={{ minHeight: '100vh' }}>
        <AppSidebar />
        <Layout className="bg-background text-foreground" style={{ minWidth: 0 }}>
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
            <AdminsPage />
          </Suspense>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
