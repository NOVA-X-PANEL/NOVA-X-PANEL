import type { ReactNode } from 'react';
import { act, renderHook } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { keys } from '@/api/queryKeys';
import { useWebSocketBridge } from '@/api/websocketBridge';

const fake = vi.hoisted(() => ({
  listeners: new Map<string, Set<(payload: unknown) => void>>(),
  isConnected: true,
  on(event: string, fn: (payload: unknown) => void) {
    if (!this.listeners.has(event)) this.listeners.set(event, new Set());
    this.listeners.get(event)!.add(fn);
  },
  off(event: string, fn: (payload: unknown) => void) {
    this.listeners.get(event)?.delete(fn);
  },
  connect: vi.fn(),
}));
vi.mock('@/api/websocket', () => ({ getSharedWebSocketClient: () => fake }));
function emit(type: string, payload: unknown = undefined) {
  for (const fn of fake.listeners.get(type) ?? []) fn(payload);
}
function setup() {
  const queryClient = new QueryClient();
  const invalidate = vi.spyOn(queryClient, 'invalidateQueries');
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  const hook = renderHook(() => useWebSocketBridge(), { wrapper });
  return { invalidate, hook };
}
beforeEach(() => {
  vi.useFakeTimers();
  fake.listeners.clear();
  fake.isConnected = true;
});
afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});
describe('websocket bridge scoped updates', () => {
  it('refreshes clients and inbounds independently, even in a burst', () => {
    const { invalidate } = setup();
    act(() => {
      emit('invalidate', { type: 'clients' });
      emit('invalidate', { type: 'inbounds' });
      emit('invalidate', { type: 'clients' });
      vi.advanceTimersByTime(200);
    });
    expect(invalidate).toHaveBeenCalledTimes(2);
    expect(invalidate).toHaveBeenCalledWith({ queryKey: keys.clients.root() });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: keys.inbounds.root() });
  });
  it.each(['traffic', 'client_stats'])('refreshes both views when %s is oversized', (type) => {
    const { invalidate } = setup();
    act(() => {
      emit('invalidate', { type });
      vi.advanceTimersByTime(200);
    });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: keys.clients.root() });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: keys.inbounds.root() });
  });
  it('refreshes scoped inbound activity without treating TUIC-only speed as a snapshot', () => {
    const { invalidate } = setup();
    act(() => {
      emit('traffic', { clientTraffics: [] });
      vi.advanceTimersByTime(200);
    });
    expect(invalidate).not.toHaveBeenCalled();
    act(() => {
      emit('traffic', { onlineClients: ['alice'] });
      vi.advanceTimersByTime(200);
    });
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: keys.clients.activeInbounds(),
      exact: true,
    });
    invalidate.mockClear();
    act(() => {
      emit('traffic', { onlineClients: [], activeInbounds: {} });
      vi.advanceTimersByTime(200);
    });
    expect(invalidate).not.toHaveBeenCalled();
  });
  it('polls active scoped queries while disconnected and catches missed events on reconnect', () => {
    const { invalidate, hook } = setup();
    fake.isConnected = false;
    act(() => {
      vi.advanceTimersByTime(5200);
    });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: keys.clients.root() });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: keys.inbounds.root() });
    invalidate.mockClear();
    fake.isConnected = true;
    act(() => {
      emit('connected');
      vi.advanceTimersByTime(200);
    });
    expect(invalidate).toHaveBeenCalledTimes(2);
    hook.unmount();
    invalidate.mockClear();
    act(() => {
      emit('connected');
      vi.advanceTimersByTime(5200);
    });
    expect(invalidate).not.toHaveBeenCalled();
  });
});
