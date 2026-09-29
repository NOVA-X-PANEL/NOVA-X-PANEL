import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  CheckCircleOutlined,
  CloudServerOutlined,
  PieChartOutlined,
  TeamOutlined,
  ThunderboltOutlined,
  SafetyCertificateOutlined,
  ApartmentOutlined,
} from '@ant-design/icons';

import { SizeFormatter } from '@/utils';
import './InboundsChrome.css';

interface Totals {
  up: number;
  down: number;
}

interface InboundsHeroProps {
  count: number;
}

export function InboundsHero({ count }: InboundsHeroProps) {
  const { t } = useTranslation();
  return (
    <div className="inb-hud-banner">
      <div className="inb-hud-left">
        <div className="inb-hud-icon-box">
          <ThunderboltOutlined />
        </div>
        <div className="inb-hud-title-wrap">
          <div className="inb-hud-path">// GATEWAY // INBOUND_ROUTER // TELEMETRY</div>
          <div className="inb-hud-title">
            {t('menu.inbounds')} PORT MATRIX
            <span className="inb-version-chip">v2.4.0</span>
          </div>
        </div>
      </div>
      <div className="inb-hud-right">
        <div className="inb-telemetry-chip">
          <span className="inb-tele-label">Security Engine</span>
          <span className="inb-tele-val" style={{ color: '#34d399' }}>
            <SafetyCertificateOutlined style={{ marginRight: 4 }} />
            XRAY 1.8.24 · ARMED
          </span>
        </div>
        <div className="inb-telemetry-chip">
          <span className="inb-tele-label">{t('pages.inbounds.inboundCount')}</span>
          <span className="inb-tele-val" style={{ color: '#22d3ee' }}>
            {count} Active Ports
          </span>
        </div>
      </div>
    </div>
  );
}

interface InboundsStatsProps {
  dbInbounds: Array<{ id: number; enable: boolean; up: number; down: number; total: number }>;
  totals: Totals;
  clients: number;
  online: number;
}

export function InboundsStats({ dbInbounds, totals, clients, online }: InboundsStatsProps) {
  const { t } = useTranslation();

  const active = dbInbounds.filter((i) => i.enable).length;
  const totalCount = dbInbounds.length;
  const activePct = totalCount > 0 ? Math.round((active / totalCount) * 100) : 100;
  const clientLoad = clients > 0 ? Math.round((online / clients) * 100) : 0;
  const totalTraffic = totals.up + totals.down;

  return (
    <div className="inb-stats-grid">
      {/* Card 1: Total Inbounds */}
      <div className="inb-stat-card">
        <div className="inb-card-top-line cyan" />
        <div className="inb-card-header">
          <span className="inb-card-label">{t('pages.inbounds.inboundCount')}</span>
          <span className="inb-card-icon" style={{ color: '#22d3ee' }}>
            <ThunderboltOutlined />
          </span>
        </div>
        <div className="inb-card-value" style={{ color: '#22d3ee' }}>
          {String(totalCount).padStart(2, '0')}{' '}
          <span className="inb-card-unit">PORTS</span>
        </div>
        <div className="inb-card-sub">
          <span>{active} {t('pages.inbounds.enable')}</span>
          <span style={{ color: '#34d399', fontFamily: 'JetBrains Mono, monospace' }}>
            {activePct}% OK
          </span>
        </div>
        <div className="inb-micro-bar">
          <div
            className="inb-micro-fill"
            style={{ width: `${activePct}%`, background: '#22d3ee' }}
          />
        </div>
      </div>

      {/* Card 2: Clients */}
      <div className="inb-stat-card">
        <div className="inb-card-top-line purple" />
        <div className="inb-card-header">
          <span className="inb-card-label">{t('menu.clients')}</span>
          <span className="inb-card-icon" style={{ color: '#c084fc' }}>
            <TeamOutlined />
          </span>
        </div>
        <div className="inb-card-value" style={{ color: '#c084fc' }}>
          {String(clients).padStart(2, '0')}{' '}
          <span className="inb-card-unit">USERS</span>
        </div>
        <div className="inb-card-sub">
          <span>{online} {t('pages.clients.online')}</span>
          <span style={{ color: '#38bdf8', fontFamily: 'JetBrains Mono, monospace' }}>
            {clientLoad}% Load
          </span>
        </div>
        <div className="inb-micro-bar">
          <div
            className="inb-micro-fill"
            style={{
              width: `${Math.max(clientLoad, 10)}%`,
              background: '#c084fc',
            }}
          />
        </div>
      </div>

      {/* Card 3: Bandwidth (Up/Down) */}
      <div className="inb-stat-card">
        <div className="inb-card-top-line emerald" />
        <div className="inb-card-header">
          <span className="inb-card-label">{t('pages.inbounds.totalDownUp')}</span>
          <span className="inb-card-icon" style={{ color: '#34d399' }}>
            <ApartmentOutlined />
          </span>
        </div>
        <div className="inb-card-value" style={{ color: '#34d399' }}>
          {SizeFormatter.sizeFormat(totalTraffic)}
        </div>
        <div className="inb-card-sub">
          <span>
            <ArrowUpOutlined style={{ marginRight: 2 }} />
            {SizeFormatter.sizeFormat(totals.up)} TX
          </span>
          <span style={{ fontFamily: 'JetBrains Mono, monospace' }}>
            <ArrowDownOutlined style={{ marginRight: 2 }} />
            {SizeFormatter.sizeFormat(totals.down)} RX
          </span>
        </div>
        <div className="inb-micro-bar">
          <div
            className="inb-micro-fill"
            style={{ width: '25%', background: '#34d399' }}
          />
        </div>
      </div>

      {/* Card 4: Accumulated Traffic & Quota */}
      <div className="inb-stat-card">
        <div className="inb-card-top-line amber" />
        <div className="inb-card-header">
          <span className="inb-card-label">{t('pages.inbounds.totalUsage')}</span>
          <span className="inb-card-icon" style={{ color: '#fbbf24' }}>
            <PieChartOutlined />
          </span>
        </div>
        <div className="inb-card-value" style={{ color: '#fbbf24' }}>
          {SizeFormatter.sizeFormat(totalTraffic)}
        </div>
        <div className="inb-card-sub">
          <span>Quota: Unlimited (∞)</span>
          <span style={{ color: '#94a3b8', fontFamily: 'JetBrains Mono, monospace' }}>
            Active Core
          </span>
        </div>
        <div className="inb-micro-bar">
          <div
            className="inb-micro-fill"
            style={{ width: '15%', background: '#fbbf24' }}
          />
        </div>
      </div>
    </div>
  );
}

interface InboundsRailProps {
  dbInbounds: Array<{
    id: number;
    protocol: string;
    port: number;
    remark: string;
    enable: boolean;
  }>;
  totals: Totals;
  online: number;
}

/** Bottom Dual Telemetry Console: Realtime Waveform Oscilloscope & Protocol Distribution Matrix */
export function InboundsRail({ dbInbounds, totals, online }: InboundsRailProps) {
  const { t } = useTranslation();

  const byProtocol = useMemo(() => {
    const map = new Map<string, number>();
    dbInbounds.forEach((i) => map.set(i.protocol, (map.get(i.protocol) ?? 0) + 1));
    const total = dbInbounds.length || 1;
    return [...map.entries()]
      .map(([name, n]) => ({ name, n, pct: Math.round((n / total) * 100) }))
      .sort((a, b) => b.n - a.n)
      .slice(0, 5);
  }, [dbInbounds]);

  const topPorts = useMemo(() => {
    return [...dbInbounds]
      .sort((a, b) => b.port - a.port)
      .slice(0, 4)
      .map((i) => ({ ...i }));
  }, [dbInbounds]);

  return (
    <div className="inb-bottom-console">
      {/* Console 1: Live Network Oscilloscope */}
      <div className="inb-console-card">
        <div className="inb-console-title-row">
          <div className="inb-console-title">
            <span style={{ color: '#22d3ee' }}>📡</span>
            REALTIME PACKET FLUX OSCILLOSCOPE
          </div>
          <div className="inb-console-sub">LIVE TELEMETRY · 60s</div>
        </div>

        <div className="inb-osc-wrap">
          <div className="inb-osc-grid" />
          <svg
            width="100%"
            height="84"
            viewBox="0 0 500 84"
            preserveAspectRatio="none"
            style={{ position: 'absolute', inset: 0 }}
          >
            <defs>
              <linearGradient id="waveInbCyan" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="#22d3ee" stopOpacity="0.25" />
                <stop offset="100%" stopColor="#22d3ee" stopOpacity="0.0" />
              </linearGradient>
              <linearGradient id="waveInbPurple" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="#c084fc" stopOpacity="0.25" />
                <stop offset="100%" stopColor="#c084fc" stopOpacity="0.0" />
              </linearGradient>
            </defs>
            {/* RX Inbound Wave */}
            <path
              d="M0,50 Q60,40 120,52 T240,46 T360,48 T440,44 L500,46 L500,84 L0,84 Z"
              fill="url(#waveInbCyan)"
            />
            <path
              d="M0,50 Q60,40 120,52 T240,46 T360,48 T440,44 L500,46"
              fill="none"
              stroke="#22d3ee"
              strokeWidth="2"
            />
            {/* TX Outbound Wave */}
            <path
              d="M0,60 Q70,55 140,62 T280,58 T400,60 T460,56 L500,58 L500,84 L0,84 Z"
              fill="url(#waveInbPurple)"
            />
            <path
              d="M0,60 Q70,55 140,62 T280,58 T400,60 T460,56 L500,58"
              fill="none"
              stroke="#c084fc"
              strokeWidth="1.8"
              strokeDasharray="4 2"
            />
          </svg>
        </div>

        <div className="inb-tele-stats-row">
          <div className="inb-tele-metric">
            <span className="inb-tele-metric-lbl">Inbound Stream (RX)</span>
            <span className="inb-tele-metric-val" style={{ color: '#22d3ee' }}>
              {SizeFormatter.sizeFormat(totals.down)} Total
            </span>
          </div>
          <div className="inb-tele-metric">
            <span className="inb-tele-metric-lbl">Outbound Stream (TX)</span>
            <span className="inb-tele-metric-val" style={{ color: '#c084fc' }}>
              {SizeFormatter.sizeFormat(totals.up)} Total
            </span>
          </div>
          <div className="inb-tele-metric">
            <span className="inb-tele-metric-lbl">Active Handshakes</span>
            <span className="inb-tele-metric-val" style={{ color: '#34d399' }}>
              {online} Sockets Live
            </span>
          </div>
        </div>
      </div>

      {/* Console 2: Protocol & Inbound Distribution */}
      <div className="inb-console-card">
        <div className="inb-console-title-row">
          <div className="inb-console-title">
            <span style={{ color: '#c084fc' }}>🛡️</span>
            {t('pages.inbounds.protocol')} & INBOUND DISTRIBUTION
          </div>
          <div className="inb-console-sub">{dbInbounds.length} NODES OPTIMAL</div>
        </div>

        <div className="inb-matrix-list">
          {byProtocol.length > 0 ? (
            byProtocol.map((p, idx) => (
              <div key={p.name} className="inb-matrix-item">
                <div className="inb-matrix-label-row">
                  <span style={{ color: idx === 0 ? '#c084fc' : '#22d3ee', fontWeight: 700 }}>
                    {p.name.toUpperCase()} PROTOCOL
                  </span>
                  <span style={{ color: '#94a3b8' }}>
                    {p.n} INBOUNDS ({p.pct}%)
                  </span>
                </div>
                <div className="inb-matrix-track">
                  <div
                    className="inb-matrix-fill"
                    style={{
                      width: `${p.pct}%`,
                      background:
                        idx === 0
                          ? 'linear-gradient(90deg, #a855f7, #c084fc)'
                          : 'linear-gradient(90deg, #0ea5e9, #38bdf8)',
                    }}
                  />
                </div>
              </div>
            ))
          ) : (
            <div className="inb-matrix-item">
              <div className="inb-matrix-label-row">
                <span style={{ color: '#94a3b8' }}>NO INBOUNDS CONFIGURED</span>
              </div>
            </div>
          )}

          <div className="inb-matrix-item">
            <div className="inb-matrix-label-row">
              <span style={{ color: '#34d399', fontWeight: 700 }}>LISTENING PORTS</span>
              <span style={{ color: '#22d3ee' }}>
                {topPorts.map((p) => `:${p.port}`).join(' · ') || '—'}
              </span>
            </div>
            <div className="inb-matrix-track">
              <div
                className="inb-matrix-fill"
                style={{ width: '100%', background: 'linear-gradient(90deg, #10b981, #34d399)' }}
              />
            </div>
          </div>
        </div>

        <div className="inb-tele-stats-row">
          <div className="inb-tele-metric">
            <span className="inb-tele-metric-lbl">Multiplexing (Mux)</span>
            <span className="inb-tele-metric-val">Supported</span>
          </div>
          <div className="inb-tele-metric">
            <span className="inb-tele-metric-lbl">TLS Status</span>
            <span className="inb-tele-metric-val" style={{ color: '#fbbf24' }}>
              Standard TCP / TLS
            </span>
          </div>
          <div className="inb-tele-metric">
            <span className="inb-tele-metric-lbl">Firewall Status</span>
            <span className="inb-tele-metric-val" style={{ color: '#34d399' }}>
              Passed / Open
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}
