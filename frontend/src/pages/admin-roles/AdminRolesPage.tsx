import { lazy, Suspense } from 'react';
import { ConfigProvider, Layout, Spin } from 'antd';

import AppSidebar from '@/layouts/AppSidebar';
import { useTheme } from '@/hooks/useTheme';

// See pages/admins/AdminsPage.tsx — same shell wrapper for the roles screen.

const AdminRolesPage = lazy(() => import('@/pg-ui/pages/_dashboard.admin-roles'));

export default function AdminRolesPageWrapper() {
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
            <AdminRolesPage />
          </Suspense>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
