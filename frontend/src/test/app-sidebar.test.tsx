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

test('renders the vertical sidebar with the brand and the status card', () => {
  const view = renderSidebar();

  // the Neon Console shell is a sider again, not the top bar
  expect(view.container.querySelector('.ant-layout-sider')).not.toBeNull();
  expect(view.container.querySelector('.nx-topbar')).toBeNull();
  expect(view.container.querySelector('.sider-brand .brand-text')?.textContent).toBe('NOVA X');
  expect(view.container.querySelector('.sider-status')).not.toBeNull();
  expect(view.container.querySelector('.sider-art')).not.toBeNull();
});

test('keeps the pinned choice made from the sidebar and restores it', () => {
  const first = renderSidebar();
  const pinButton = screen.getByRole('button', { name: 'Pin sidebar' });

  expect(pinButton.getAttribute('aria-pressed')).toBe('false');

  fireEvent.click(pinButton);

  expect(localStorage.getItem('sidebar-pinned')).toBe('true');

  first.unmount();

  renderSidebar();

  expect(screen.getByRole('button', { name: 'Pin sidebar' }).getAttribute('aria-pressed')).toBe(
    'true',
  );
});

test('unpinning from the sidebar clears the stored choice', () => {
  renderSidebar();

  fireEvent.click(screen.getByRole('button', { name: 'Pin sidebar' }));
  fireEvent.click(screen.getByRole('button', { name: 'Pin sidebar' }));

  expect(localStorage.getItem('sidebar-pinned')).toBe('false');
});

test('labels the palette shortcut with the modifier the platform actually uses', () => {
  const view = renderSidebar();
  const chip = view.container.querySelector('.sidebar-command-kbd');
  expect(chip?.textContent).toBe('CtrlK');
});
