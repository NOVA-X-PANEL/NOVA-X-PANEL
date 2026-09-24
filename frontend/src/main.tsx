import { createRoot } from 'react-dom/client';
import { RouterProvider } from 'react-router/dom';
import { message } from 'antd';
import 'antd/dist/reset.css';
import '@/styles/utils.css';
import '@/styles/page-shell.css';
import '@/styles/page-cards.css';
import '@/styles/nova-glass.css';
import '@/styles/nova-skin.css';
// The Tailwind theme and utilities, loaded ONCE for the whole app.
//
// This must be here and not in a page. The ported pg-ui screens are the only
// Tailwind consumers (Button, Card, Dialog, Switch, Table, …), and when
// pasarguard.css was imported only from those pages the bundler hoisted it into
// a shared CSS chunk and attached that chunk to only some of the importers. The
// My API page — which imports it directly and has no other pg-ui page beside it —
// was left out: its chunk emitted `/* empty css */` placeholders and referenced
// only the panel's own stylesheet. Every component on that page therefore fell
// back to unstyled HTML, and the Radix Dialog, which has no positioning of its
// own, rendered off-screen. The page looked like it ignored every click.
//
// Importing it at the entry point puts the layer in the one chunk that every
// route loads, so no page can miss it. That is safe by design: pasarguard.css
// deliberately omits Tailwind's Preflight (see its header comment), so it adds
// the theme variables and utility classes without resetting anything, and no
// antd-based screen uses those class names.
import '@/pg-ui/styles/pasarguard.css';
// The phone layer for the ported pg-ui screens, for the same reason: it is scoped
// to `.pg-admin-page` / `.pg-dialog`, so it is inert on the antd screens, but a
// pg-ui page that does not receive it loses the small-screen fixes — including
// the rule that keeps a long dialog scrollable instead of running off the top of
// a phone viewport.
import '@/styles/pg-admin-mobile.css';

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
