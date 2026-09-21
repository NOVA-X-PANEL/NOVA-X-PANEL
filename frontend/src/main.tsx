import { createRoot } from 'react-dom/client';
import { RouterProvider } from 'react-router/dom';
import { message } from 'antd';
import 'antd/dist/reset.css';
import '@/styles/utils.css';
import '@/styles/page-shell.css';
import '@/styles/page-cards.css';
import '@/styles/nova-glass.css';

import { setupHttp } from '@/api/http-init';
import { readyI18n } from '@/i18n/react';
// Registers the ported admin screens' translation bundle. Its module body hooks
// i18next, so it must be imported before readyI18n() initialises i18n; without
// this import the whole bundle is tree-shaken away and every pg-ui string
// renders as a raw key ("admins.createAdmin").
import '@/pg-ui/i18n/admin-pages-bridge';
import { ThemeProvider } from '@/hooks/useTheme';
import { QueryProvider } from '@/api/QueryProvider';
import { router } from '@/routes';
import { Toaster } from '@/pg-ui/components/ui/sonner';

setupHttp();

const messageContainer = document.getElementById('message');
if (messageContainer) {
  message.config({ getContainer: () => messageContainer });
}

readyI18n().then(() => {
  const root = document.getElementById('app');
  if (root) {
    createRoot(root).render(
      <ThemeProvider>
        <QueryProvider>
          <RouterProvider router={router} />
          <Toaster richColors closeButton />
        </QueryProvider>
      </ThemeProvider>,
    );
  }
});
