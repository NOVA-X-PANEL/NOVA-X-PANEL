// Adaptive route and chunk preloader for NOVA X PANEL.
// Preloads chunks in idle time and on sidebar hover to make page switching instant.

const routeLoaders: Record<string, () => Promise<unknown>> = {
  '/': () => import('@/pages/index/IndexPage'),
  '/inbounds': () => import('@/pages/inbounds/InboundsPage'),
  '/clients': () => import('@/pages/clients/ClientsPage'),
  '/groups': () => import('@/pages/groups/GroupsPage'),
  '/nodes': () => import('@/pages/nodes/NodesPage'),
  '/admins': () => import('@/pages/admins/AdminsPage'),
  '/my-api': () => import('@/pages/my-api/MyApiPage'),
  '/admin-roles': () => import('@/pages/admin-roles/AdminRolesPage'),
  '/hosts': () => import('@/pages/hosts/HostsPage'),
  '/settings': () => import('@/pages/settings/SettingsPage'),
  '/xray': () => import('@/pages/xray/XrayPage'),
  '/outbound': () => import('@/pages/xray/XrayPage'),
  '/routing': () => import('@/pages/xray/XrayPage'),
  '/api-docs': () => import('@/pages/api-docs/ApiDocsPage'),
};

const preloadedRoutes = new Set<string>();

export function preloadRoute(path: string): void {
  const basePath = path.split('#')[0].split('?')[0] || '/';
  if (preloadedRoutes.has(basePath)) return;
  const loader = routeLoaders[basePath];
  if (loader) {
    preloadedRoutes.add(basePath);
    loader().catch(() => {
      preloadedRoutes.delete(basePath);
    });
  }
}

/** Preload top frequented pages first, then idle-preload remainder */
export function preloadAllRoutes(): void {
  // Core pages first
  const highPriority = ['/inbounds', '/clients', '/settings'];
  highPriority.forEach((p) => preloadRoute(p));

  const scheduleIdle =
    typeof window !== 'undefined' && 'requestIdleCallback' in window
      ? window.requestIdleCallback
      : (cb: () => void) => setTimeout(cb, 1200);

  scheduleIdle(() => {
    Object.keys(routeLoaders).forEach((path) => {
      preloadRoute(path);
    });
  });
}
