import { describe, expect, it } from 'vitest';

import { canAccessRoute, canReadResourcePage, hasPermission } from '@/pg-ui/utils/rbac';

// RBAC route gating. The backend lets a role holding the "simple read" open the
// Inbounds and Clients pages (their list endpoints accept read or read_simple),
// so the menu and the router must agree with it. Before this, a role whose role
// document granted only inbound read_simple — the seeded Operator — could not
// reach a page its own permissions were meant to allow.

const admin = (permissions: Record<string, unknown>) => ({ role: { permissions } });

describe('route access', () => {
  it('grants the inbounds page on the full read', () => {
    expect(canAccessRoute(admin({ inbounds: { read: true } }), '/inbounds')).toBe(true);
  });

  it('grants the inbounds page on the simple read', () => {
    expect(canAccessRoute(admin({ inbounds: { read_simple: true } }), '/inbounds')).toBe(true);
  });

  it('grants the clients page on the simple read', () => {
    expect(canAccessRoute(admin({ users: { read_simple: { scope: 2 } } }), '/clients')).toBe(true);
  });

  it('still refuses a page with no grant at all', () => {
    expect(canAccessRoute(admin({ users: { read_simple: true } }), '/inbounds')).toBe(false);
    expect(canAccessRoute(admin({ inbounds: { read: true } }), '/clients')).toBe(false);
  });

  it('opens the API page only for accounts granted API access', () => {
    expect(canAccessRoute({ role: { slug: 'owner' } }, '/my-api')).toBe(true);
    expect(canAccessRoute({ api_access: true }, '/my-api')).toBe(true);
    expect(canAccessRoute({ role: { permissions: { inbounds: { read: true } } } }, '/my-api')).toBe(
      false,
    );
  });

  it("keeps the API page out of a tokenless account's fallback route", () => {
    // An operator with no other grant must not be silently dropped on the API page.
    const operator = { role: { permissions: { inbounds: { read_simple: true } } } };
    expect(canAccessRoute(operator, '/my-api')).toBe(false);
    expect(canAccessRoute(operator, '/inbounds')).toBe(true);
  });

  it('treats the owner role as full access', () => {
    expect(canAccessRoute({ role: { slug: 'owner' } }, '/clients')).toBe(true);
    expect(canAccessRoute({ role: { ownerRole: true } }, '/inbounds')).toBe(true);
  });

  it('reads the camelCase spellings the older role documents used', () => {
    expect(
      canAccessRoute(admin({ inbounds: { readSimple: true, viewSimple: true } }), '/inbounds'),
    ).toBe(true);
    expect(canReadResourcePage(admin({ users: { viewSimpleList: true } }), 'users')).toBe(true);
  });

  it('keeps scope "none" out', () => {
    expect(hasPermission(admin({ users: { read: { scope: 0 } } }), 'users', 'read')).toBe(false);
    expect(canAccessRoute(admin({ users: { read: { scope: 0 } } }), '/clients')).toBe(false);
  });
});
