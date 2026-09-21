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

test('renders the top navigation bar instead of a vertical sidebar', () => {
  const view = renderSidebar();

  expect(view.container.querySelector('.nx-topbar')).not.toBeNull();
  expect(view.container.querySelector('.ant-layout-sider')).toBeNull();
  expect(view.container.querySelector('.nx-brand-text')?.textContent).toBe('NOVA X');
  expect(view.container.querySelector('.nx-nav')).not.toBeNull();
});

test('keeps the pinned choice made from the top bar and restores it', () => {
  const first = renderSidebar();
  const pinButton = screen.getByRole('button', { name: 'Pin sidebar' });

  expect(pinButton.getAttribute('aria-pressed')).toBe('false');

  fireEvent.click(pinButton);

  expect(localStorage.getItem('sidebar-pinned')).toBe('true');

  first.unmount();

  const second = renderSidebar();
  const restoredPin = screen.getByRole('button', { name: 'Pin sidebar' });

  expect(restoredPin.getAttribute('aria-pressed')).toBe('true');
  expect(second.container.querySelector('.nx-topbar')).not.toBeNull();
});

test('unpinning from the top bar clears the stored choice', () => {
  renderSidebar();
  const pinButton = screen.getByRole('button', { name: 'Pin sidebar' });

  fireEvent.click(pinButton);
  fireEvent.click(screen.getByRole('button', { name: 'Pin sidebar' }));

  expect(localStorage.getItem('sidebar-pinned')).toBe('false');
});

test('labels the palette shortcut with the modifier the platform actually uses', () => {
  const view = renderSidebar();
  const chip = view.container.querySelector('.sidebar-command-kbd');
  expect(chip?.textContent).toBe('CtrlK');
});
