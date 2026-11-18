import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { getSharedWebSocketClient } from '@/api/websocket';
import { keys } from '@/api/queryKeys';
type Handler = (payload: unknown) => void;
export function useWebSocketBridge() {
  const queryClient = useQueryClient();
  useEffect(() => {
    const client = getSharedWebSocketClient();
    // Coalesce a burst per resource without letting a clients invalidate erase
    // an inbounds invalidate (or endlessly postpone either during a busy tick).
    const timers = new Map<string, number>();
    const schedule = (type: string) => {
      if (timers.has(type)) return;
      timers.set(
        type,
        window.setTimeout(() => {
          timers.delete(type);
          switch (type) {
            case 'inbounds':
              void queryClient.invalidateQueries({ queryKey: keys.inbounds.root() });
              break;
            case 'clients':
              void queryClient.invalidateQueries({ queryKey: keys.clients.root() });
              break;
            case 'activeInbounds':
              void queryClient.invalidateQueries({
                queryKey: keys.clients.activeInbounds(),
                exact: true,
              });
              break;
            case 'nodes':
              void queryClient.invalidateQueries({ queryKey: keys.nodes.root() });
              break;
            case 'outbounds':
              void queryClient.invalidateQueries({
                queryKey: keys.xray.outboundsTraffic(),
                exact: true,
              });
              break;
          }
        }, 200),
      );
    };
    const onInvalidate: Handler = (payload) => {
      const p = payload as { type?: string } | undefined;
      switch (p?.type) {
        case 'clients':
        case 'inbounds':
        case 'nodes':
        case 'outbounds':
          schedule(p.type);
          break;
        case 'traffic':
        case 'client_stats':
          // The hub substitutes an invalidate for oversized events. Both
          // client and inbound views depend on these otherwise-dropped events.
          schedule('clients');
          schedule('inbounds');
          break;
      }
    };
    const onTraffic: Handler = (payload) => {
      const p = payload as { onlineClients?: unknown; activeInbounds?: unknown } | undefined;
      // Scoped events omit the global inbound activity map. Re-fetch the
      // authorized endpoint instead; TUIC-only speed samples have no online set.
      if (Array.isArray(p?.onlineClients) && p?.activeInbounds === undefined) {
        schedule('activeInbounds');
      }
    };
    const onConnected: Handler = () => {
      // Catch mutations missed while disconnected.
      schedule('clients');
      schedule('inbounds');
    };
    const onOutbounds: Handler = (payload) => {
      if (!Array.isArray(payload)) return;
      queryClient.setQueryData(keys.xray.outboundsTraffic(), payload);
    };
    const onNodes: Handler = (payload) => {
      if (!Array.isArray(payload)) return;
      queryClient.setQueryData(keys.nodes.list(), payload);
    };
    const onInbounds: Handler = (payload) => {
      if (!Array.isArray(payload)) return;
      queryClient.setQueryData(keys.inbounds.slim(), payload);
    };
    client.on('invalidate', onInvalidate);
    client.on('traffic', onTraffic);
    client.on('connected', onConnected);
    client.on('outbounds', onOutbounds);
    client.on('nodes', onNodes);
    client.on('inbounds', onInbounds);
    client.connect();
    // Keep status endpoints live if the socket is down, even though their
    // queries otherwise have infinite staleTime. Only active queries refetch.
    const fallback = window.setInterval(() => {
      if (!client.isConnected) {
        schedule('clients');
        schedule('inbounds');
      }
    }, 5000);
    return () => {
      client.off('invalidate', onInvalidate);
      client.off('traffic', onTraffic);
      client.off('connected', onConnected);
      client.off('outbounds', onOutbounds);
      client.off('nodes', onNodes);
      client.off('inbounds', onInbounds);
      window.clearInterval(fallback);
      for (const timer of timers.values()) window.clearTimeout(timer);
    };
  }, [queryClient]);
}
