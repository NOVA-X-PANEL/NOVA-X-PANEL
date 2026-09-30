import { useEffect } from 'react';
import { Outlet } from 'react-router';

import { useWebSocketBridge } from '@/api/websocketBridge';
import { usePageTitle } from '@/hooks/usePageTitle';
import CommandPalette from '@/components/command-palette/CommandPalette';
import { preloadAllRoutes } from '@/utils/routePreload';

export default function PanelLayout() {
  useWebSocketBridge();
  usePageTitle();

  useEffect(() => {
    preloadAllRoutes();
  }, []);

  return (
    <>
      <Outlet />
      <CommandPalette />
    </>
  );
}
