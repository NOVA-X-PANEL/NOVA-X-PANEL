import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import NeonLandscape from '@/components/NeonLandscape';
import {
  CloudDownloadOutlined,
  CloudUploadOutlined,
  DownloadOutlined,
  PlusOutlined,
  ReloadOutlined,
  SlidersOutlined,
  TeamOutlined,
  ThunderboltOutlined,
  WifiOutlined,
} from '@ant-design/icons';

/**
 * Page chrome for the Clients screen, matching the Neon Console reference: a
 * hero banner with two headline metrics, and a right-hand rail holding quick
 * actions and a protocol donut. Presentational only — the page owns the data and
 * the handlers.
 */

interface ClientsHeroProps {
  total: number;
  online: number;
}

export function ClientsHero({ total, online }: ClientsHeroProps) {
  const { t } = useTranslation();
  return (
    <div className="nc-hero">
      <NeonLandscape variant="panel" />
      <span className="nc-hero-icon">
        <TeamOutlined />
      </span>
      <div className="nc-hero-text">
        <div className="nc-hero-title">{t('menu.clients')}</div>
        <div className="nc-hero-sub">{t('menu.clients')}</div>
      </div>
      <div className="nc-hero-metrics">
        <div className="nc-hero-metric">
          <div className="nc-hero-metric-label">{t('clients')}</div>
          <div className="nc-hero-metric-value">{total}</div>
        </div>
        <div className="nc-hero-metric">
          <div className="nc-hero-metric-label">{t('online')}</div>
          <div className="nc-hero-metric-value nc-hero-metric-value--on">{online}</div>
        </div>
      </div>
    </div>
  );
}

interface ClientsRailProps {
  protocols: Array<{ name: string; n: number }>;
  total: number;
  onAdd: () => void;
  onImport: () => void;
  onExport: () => void;
  onResetTraffic: () => void;
  onDefaults: () => void;
}

const DONUT_PALETTE = ['#2563ff', '#7140ff', '#00bfff', '#13d6b0', '#f13b96'];

/** Right-hand rail: quick actions and a protocol donut. */
export function ClientsRail({
  protocols,
  total,
  onAdd,
  onImport,
  onExport,
  onResetTraffic,
}: ClientsRailProps) {
  const { t } = useTranslation();

  const slices = useMemo(() => {
    const sum = protocols.reduce((acc, p) => acc + p.n, 0) || 1;
    return protocols
      .map((p) => ({ ...p, pct: Math.round((p.n / sum) * 100) }))
      .sort((a, b) => b.n - a.n)
      .slice(0, 5);
  }, [protocols]);

  const donut = useMemo(() => {
    const arcs = slices.map((p, idx) => ({
      color: DONUT_PALETTE[idx % DONUT_PALETTE.length],
      pct: p.pct,
    }));
    const stops = arcs.map((a, idx) => {
      const from = arcs.slice(0, idx).reduce((acc, x) => acc + x.pct, 0);
      return `${a.color} ${from}% ${from + a.pct}%`;
    });
    const used = arcs.reduce((acc, a) => acc + a.pct, 0);
    if (used < 100) stops.push(`rgba(255,255,255,.10) ${used}% 100%`);
    return `conic-gradient(${stops.join(',')})`;
  }, [slices]);

  const actions = [
    { key: 'add', icon: <PlusOutlined />, label: t('pages.clients.addClients'), onClick: onAdd },
    {
      key: 'import',
      icon: <CloudUploadOutlined />,
      label: t('pages.clients.importClients'),
      onClick: onImport,
    },
    {
      key: 'export',
      icon: <CloudDownloadOutlined />,
      label: t('pages.clients.exportClients'),
      onClick: onExport,
    },
    {
      key: 'reset',
      icon: <ReloadOutlined />,
      label: t('pages.clients.resetAllTrafficsTitle'),
      onClick: onResetTraffic,
    },
    {
      key: 'defaults',
      icon: <SlidersOutlined />,
      label: t('pages.clients.defaultLimits'),
      onClick: onDefaults,
    },
  ];

  return (
    <aside className="nc-rail">
      <div className="nc-card">
        <div className="nc-card-head">
          <ThunderboltOutlined />
          <span>{t('pages.clients.actions')}</span>
        </div>
        <div className="nc-actions">
          {actions.map((a) => (
            <button key={a.key} type="button" className="nc-action" onClick={a.onClick}>
              <span className="nc-action-icon">{a.icon}</span>
              <span className="nc-action-label">{a.label}</span>
              <span className="nc-action-chevron" aria-hidden="true">
                <DownloadOutlined />
              </span>
            </button>
          ))}
        </div>
      </div>

      <div className="nc-card">
        <div className="nc-card-head">
          <WifiOutlined />
          <span>{t('pages.inbounds.protocol')}</span>
        </div>
        <div className="nc-donut-wrap">
          <span className="nc-donut" style={{ background: donut }}>
            <span className="nc-donut-hole">
              <span className="nc-donut-num">{total}</span>
              <span className="nc-donut-cap">{t('menu.clients')}</span>
            </span>
          </span>
          <ul className="nc-legend">
            {slices.length === 0 && (
              <li>
                <span className="nc-legend-name">{t('noData')}</span>
              </li>
            )}
            {slices.map((p, idx) => (
              <li key={p.name}>
                <span
                  className="nc-legend-dot"
                  style={{ background: DONUT_PALETTE[idx % DONUT_PALETTE.length] }}
                />
                <span className="nc-legend-name">{p.name}</span>
                <span className="nc-legend-pct">{p.pct}%</span>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </aside>
  );
}
