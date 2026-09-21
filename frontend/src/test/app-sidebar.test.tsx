import { fireEvent, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, expect, test, vi } from 'vitest';

import AppSidebar from '@/layouts/AppSidebar';
import { renderWithProviders } from './test-utils';

vi.mock('@/api/queries/useAllSettings', () => ({
  useAllSettings: () => ({ allSetting: {} }),
}));

afterEach(() => {
  localStorage.clear();
});

function renderSidebar() {
  return renderWithProviders(
    <MemoryRouter>
      <AppSidebar />
    </MemoryRouter>,
  );
}

test('renders the vertical sidebar shell with its brand', () => {
  const view = renderSidebar();

  // the Neon Console shell is a sider again, not the top bar
  expect(view.container.querySelector('.ant-layout-sider')).not.toBeNull();
  expect(view.container.querySelector('.nx-topbar')).toBeNull();
  expect(view.container.querySelector('.sider-brand .brand-mark')).not.toBeNull();
});

test('expands on hover, shows the status card and artwork, and pins', () => {
  const view = renderSidebar();
  const sidebar = view.container.querySelector('.ant-layout-sider');
  const sidebarRoot = view.container.querySelector('.ant-sidebar');

  // a stored choice wins, but a fresh visit starts expanded
  expect(sidebar?.classList.contains('ant-layout-sider-collapsed')).toBe(false);

  fireEvent.mouseEnter(sidebarRoot!);

  expect(view.container.querySelector('.sider-brand .brand-text')?.textContent).toBe('NOVA X');
  expect(view.container.querySelector('.sider-status')).not.toBeNull();
  expect(view.container.querySelector('.sider-art')).not.toBeNull();

  const pinButton = screen.getByRole('button', { name: 'Pin sidebar' });
  fireEvent.click(pinButton);
  fireEvent.mouseLeave(sidebarRoot!);

  expect(sidebar?.classList.contains('ant-layout-sider-collapsed')).toBe(false);
  expect(localStorage.getItem('sidebar-pinned')).toBe('true');

  view.unmount();

  const second = renderSidebar();
  const restored = second.container.querySelector('.ant-layout-sider');
  expect(restored?.classList.contains('ant-layout-sider-collapsed')).toBe(false);
});

test('unpinning returns to the compact rail', () => {
  const view = renderSidebar();
  const sidebarRoot = view.container.querySelector('.ant-sidebar');
  const sidebar = view.container.querySelector('.ant-layout-sider');

  fireEvent.mouseEnter(sidebarRoot!);
  // starts pinned, so unpinning is what collapses it
  fireEvent.click(screen.getByRole('button', { name: 'Pin sidebar' }));
  fireEvent.mouseLeave(sidebarRoot!);

  expect(sidebar?.classList.contains('ant-layout-sider-collapsed')).toBe(true);
  expect(localStorage.getItem('sidebar-pinned')).toBe('false');
});

test('labels the palette shortcut with the modifier the platform actually uses', () => {
  const view = renderSidebar();
  const chip = view.container.querySelector('.sidebar-command-kbd');
  expect(chip?.textContent).toBe('CtrlK');
});
