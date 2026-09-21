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

test('renders the vertical sidebar expanded by default, with brand, status and artwork', () => {
  const view = renderSidebar();

  // the Neon Console shell is a sider again, not the top bar
  expect(view.container.querySelector('.nx-topbar')).toBeNull();
  expect(view.container.querySelector('.sider-brand .brand-mark')).not.toBeNull();
  expect(view.container.querySelector('.sider-brand .brand-text')?.textContent).toBe('NOVA X');
  expect(view.container.querySelector('.sider-status')).not.toBeNull();
  expect(view.container.querySelector('.sider-art')).not.toBeNull();
});

test('unpinning collapses the rail and the choice persists', () => {
  const first = renderSidebar();
  const sidebar = first.container.querySelector('.ant-layout-sider');
  const sidebarRoot = first.container.querySelector('.ant-sidebar');

  expect(sidebar?.classList.contains('ant-layout-sider-collapsed')).toBe(false);

  fireEvent.click(screen.getByRole('button', { name: 'Pin sidebar' }));
  fireEvent.mouseLeave(sidebarRoot!);

  expect(sidebar?.classList.contains('ant-layout-sider-collapsed')).toBe(true);
  expect(localStorage.getItem('sidebar-pinned')).toBe('false');

  first.unmount();

  const second = renderSidebar();
  expect(
    second.container
      .querySelector('.ant-layout-sider')
      ?.classList.contains('ant-layout-sider-collapsed'),
  ).toBe(true);
});

test('pin from the rail keeps it expanded after the pointer leaves', () => {
  localStorage.setItem('sidebar-pinned', 'false');
  const view = renderSidebar();
  const sidebar = view.container.querySelector('.ant-layout-sider');
  const sidebarRoot = view.container.querySelector('.ant-sidebar');

  expect(sidebar?.classList.contains('ant-layout-sider-collapsed')).toBe(true);

  fireEvent.mouseEnter(sidebarRoot!);
  fireEvent.click(screen.getByRole('button', { name: 'Pin sidebar' }));
  fireEvent.mouseLeave(sidebarRoot!);

  expect(sidebar?.classList.contains('ant-layout-sider-collapsed')).toBe(false);
  expect(localStorage.getItem('sidebar-pinned')).toBe('true');
});

test('labels the palette shortcut with the modifier the platform actually uses', () => {
  const view = renderSidebar();
  const chip = view.container.querySelector('.sidebar-command-kbd');
  expect(chip?.textContent).toBe('CtrlK');
});
