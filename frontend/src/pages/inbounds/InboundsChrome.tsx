import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  BarsOutlined,
  CheckCircleOutlined,
  CloudServerOutlined,
  PieChartOutlined,
  StopOutlined,
  TeamOutlined,
} from '@ant-design/icons';

import { SizeFormatter } from '@/utils';

/**
 * Page chrome for the Inbounds screen, matching the Neon Console reference: a
 * hero banner, a row of icon-tile stat cards, and a right-hand analytics rail.
 * Presentational only — the page owns the data.
 */

const DONUT_PALETTE = ['#2563ff', '#7140ff', '#00bfff', '#13d6b0', '#f13b96'];

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
    <div className="nc-hero">
      <span className="nc-hero-icon">
        <CloudServerOutlined />
      </span>
      <div className="nc-hero-text">
        <div className="nc-hero-title">{t('menu.inbounds')}</div>
        <div className="nc-hero-sub">{t('menu.inbounds')}</div>
      </div>
      <div className="nc-hero-metric">
        <div className="nc-hero-metric-label">{t('pages.inbounds.inboundCount')}</div>
        <div className="nc-hero-metric-value">{count}</div>
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
  const inactive = dbInbounds.length - active;

  const cards = [
    {
      key: 'total',
      label: t('pages.inbounds.inboundCount'),
      value: String(dbInbounds.length),
      icon: <CloudServerOutlined />,
      tint: 'nc-tint-blue',
      foot: null as string | null,
    },
    {
      key: 'active',
      label: t('pages.inbounds.enable'),
      value: String(active),
      icon: <CheckCircleOutlined />,
      tint: 'nc-tint-teal',
      foot: null,
    },
    {
      key: 'inactive',
      label: t('disabled'),
      value: String(inactive),
      icon: <StopOutlined />,
      tint: 'nc-tint-pink',
      foot: null,
    },
    {
      key: 'traffic',
      label: t('pages.inbounds.totalUsage'),
      value: SizeFormatter.sizeFormat(totals.up + totals.down),
      icon: <PieChartOutlined />,
      tint: 'nc-tint-violet',
      foot: `${SizeFormatter.sizeFormat(totals.up)} ↑ · ${SizeFormatter.sizeFormat(totals.down)} ↓`,
    },
    {
      key: 'clients',
      label: t('menu.clients'),
      value: String(clients),
      icon: <TeamOutlined />,
      tint: 'nc-tint-blue',
      foot: `${online} ${t('pages.clients.online')}`,
    },
  ];

  return (
    <div className="nc-stats">
      {cards.map((c) => (
        <div key={c.key} className="nc-stat">
          <span className={`nc-stat-icon ${c.tint}`}>{c.icon}</span>
          <div className="nc-stat-label">{c.label}</div>
          <div className="nc-stat-value">{c.value}</div>
          {c.foot && <div className="nc-stat-foot">{c.foot}</div>}
        </div>
      ))}
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

/** Right-hand analytics rail: usage bars, protocol donut and a status list. */
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

  // A donut built from conic-gradient stops, one arc per protocol.
  const donut = useMemo(() => {
    const arcs = byProtocol.map((p, idx) => ({
      color: DONUT_PALETTE[idx % DONUT_PALETTE.length],
      pct: p.pct,
    }));
    const stops = arcs.map((a, idx) => {
      const from = arcs.slice(0, idx).reduce((sum, x) => sum + x.pct, 0);
      return `${a.color} ${from}% ${from + a.pct}%`;
    });
    const used = arcs.reduce((sum, a) => sum + a.pct, 0);
    if (used < 100) stops.push(`rgba(255,255,255,.10) ${used}% 100%`);
    return `conic-gradient(${stops.join(',')})`;
  }, [byProtocol]);

  const topPorts = [...dbInbounds]
    .sort((a, b) => b.port - a.port)
    .slice(0, 5)
    .map((i) => ({ ...i }));

  return (
    <aside className="nc-rail">
      <div className="nc-card">
        <div className="nc-card-head">
          <PieChartOutlined />
          <span>{t('pages.inbounds.totalUsage')}</span>
        </div>
        <div className="nc-rail-bars">
          <div className="nc-rail-bar">
            <span className="nc-rail-bar-label">
              <ArrowUpOutlined /> {t('pages.inbounds.totalDownUp')}
            </span>
            <span className="nc-rail-bar-value">{SizeFormatter.sizeFormat(totals.up)}</span>
          </div>
          <div className="nc-rail-bar">
            <span className="nc-rail-bar-label">
              <ArrowDownOutlined />
            </span>
            <span className="nc-rail-bar-value">{SizeFormatter.sizeFormat(totals.down)}</span>
          </div>
          <div className="nc-rail-bar">
            <span className="nc-rail-bar-label">
              <TeamOutlined /> {t('pages.clients.online')}
            </span>
            <span className="nc-rail-bar-value">{online}</span>
          </div>
        </div>
      </div>

      <div className="nc-card">
        <div className="nc-card-head">
          <BarsOutlined />
          <span>{t('pages.inbounds.protocol')}</span>
        </div>
        <div className="nc-donut-wrap">
          <span className="nc-donut" style={{ background: donut }}>
            <span className="nc-donut-hole">
              <span className="nc-donut-num">{dbInbounds.length}</span>
              <span className="nc-donut-cap">{t('pages.inbounds.inboundCount')}</span>
            </span>
          </span>
          <ul className="nc-legend">
            {byProtocol.map((p, idx) => (
              <li key={p.name}>
                <span
                  className="nc-legend-dot"
                  style={{ background: palette[idx % palette.length] }}
                />
                <span className="nc-legend-name">{p.name}</span>
                <span className="nc-legend-pct">{p.pct}%</span>
              </li>
            ))}
          </ul>
        </div>
      </div>

      <div className="nc-card">
        <div className="nc-card-head">
          <CloudServerOutlined />
          <span>{t('pages.inbounds.port')}</span>
        </div>
        <ul className="nc-rail-list">
          {topPorts.map((i) => (
            <li key={i.id}>
              <span className={`nc-rail-dot${i.enable ? ' is-on' : ''}`} />
              <span className="nc-rail-name">{i.remark || `#${i.id}`}</span>
              <span className="nc-rail-port">{i.port}</span>
            </li>
          ))}
        </ul>
      </div>
    </aside>
  );
}
