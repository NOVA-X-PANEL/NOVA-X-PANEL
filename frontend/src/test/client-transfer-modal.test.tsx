import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import ClientTransferModal from '@/pages/clients/ClientTransferModal';

const { useGetAdminsSimple } = vi.hoisted(() => ({ useGetAdminsSimple: vi.fn() }));
vi.mock('@/pg-ui/service/api', () => ({ useGetAdminsSimple }));

const admins = [
  { id: 1, username: 'owner', status: 'active', role: { ownerRole: true } },
  { id: 2, username: 'already-owner', status: 'active', role: { ownerRole: false } },
  { id: 3, username: 'disabled-admin', status: 'disabled', role: { ownerRole: false } },
  { id: 4, username: 'destination-admin', status: 'active', role: { ownerRole: false } },
];

describe('ClientTransferModal', () => {
  beforeEach(() => {
    useGetAdminsSimple.mockReturnValue({ data: { admins }, isLoading: false, error: null });
  });

  it('offers only active non-owner admins other than the current owner', async () => {
    const onSubmit = vi.fn();
    render(
      <ClientTransferModal
        open
        email="alice@example.test"
        currentOwnerId={2}
        onCancel={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    await userEvent.click(screen.getByRole('combobox'));
    expect(await screen.findByText('destination-admin')).toBeTruthy();
    expect(screen.queryByText('owner')).toBeNull();
    expect(screen.queryByText('already-owner')).toBeNull();
    expect(screen.queryByText('disabled-admin')).toBeNull();

    await userEvent.click(screen.getByText('destination-admin'));
    await userEvent.click(screen.getByRole('button', { name: 'Confirm' }));
    await waitFor(() => expect(onSubmit).toHaveBeenCalledExactlyOnceWith(4));
  });
});
