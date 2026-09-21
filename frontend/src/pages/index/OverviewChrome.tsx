import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  ApiOutlined,
  ClockCircleOutlined,
  DashboardOutlined,
  DatabaseOutlined,
  HddOutlined,
  RocketOutlined,
  SmileOutlined,
  SwapOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';

import { CPUFormatter, SizeFormatter, TimeFormatter } from '@/utils';
import type { Status } from '@/models/status';

/**
 * Page chrome for the Overview screen, matching the Neon Console reference: a
 * welcome banner, a row of icon-tile metric cards and a right-hand rail with
 * server information plus donut gauges. Presentational only — the page owns the
 * data and its polling.
 */

interface OverviewHeroProps {
  panelVersion: string;
}

export function OverviewHero({ panelVersion }: OverviewHeroProps) {
  const { t } = useTranslation();
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    const id = window.setInterval(() => setNow(new Date()), 30_000);
    return () => window.clearInterval(id);
  }, []);

  const date = useMemo(
    () => new Intl.DateTimeFormat('fa-IR', { dateStyle: 'short' }).format(now),
    [now],
  );
  const time = useMemo(
    () => new Intl.DateTimeFormat('fa-IR', { hour: '2-digit', minute: '2-digit' }).format(now),
    [now],
  );

  return (
    <div className="nc-hero">
      <span className="nc-hero-badge" aria-hidden="true">
        <SmileOutlined />
      </span>
      <div className="nc-hero-text">
        <div className="nc-hero-title">{t('menu.dashboard')}</div>
        <div className="nc-hero-sub">{t('pages.index.systemHistoryTitle')}</div>
      </div>
      <div className="nc-hero-metrics">
        <div className="nc-hero-clock">
          <div className="nc-hero-clock-date">{date}</div>
          <div className="nc-hero-clock-time" dir="ltr">
            {time}
          </div>
        </div>
        <div className="nc-hero-metric">
          <div className="nc-hero-metric-label">{t('pages.index.currentPanelVersion')}</div>
          <div className="nc-hero-metric-value">v{panelVersion}</div>
        </div>
      </div>
    </div>
  );
}

interface OverviewStatsProps {
  status: Status;
}

/** Four icon-tile metric cards built from the status payload. */
export function OverviewStats({ status }: OverviewStatsProps) {
  const { t } = useTranslation();
  const totalTraffic = status.netTraffic.sent + status.netTraffic.recv;
  const sockets = status.tcpCount + status.udpCount;

  const cards = [
    {
      key: 'traffic',
      icon: <SwapOutlined />,
      tint: 'nc-tint-violet',
      label: t('pages.index.overallSpeed'),
      value: SizeFormatter.sizeFormat(totalTraffic),
      foot: `${t('pages.index.sent')} ${SizeFormatter.sizeFormat(status.netTraffic.sent)}`,
      border: '#4c25c8',
    },
    {
      key: 'sockets',
      icon: <ApiOutlined />,
      tint: 'nc-tint-teal',
      label: t('pages.index.connectionCount'),
      value: String(sockets),
      foot: `TCP ${status.tcpCount} · UDP ${status.udpCount}`,
      border: '#096a85',
    },
    {
      key: 'cpu',
      icon: <DashboardOutlined />,
      tint: 'nc-tint-blue',
      label: t('pages.index.cpu'),
      value: `${status.cpu.percent.toFixed(0)}%`,
      foot: `${CPUFormatter.cpuCoreFormat(status.cpuCores)} / ${status.logicalPro}T`,
      border: '#1266d8',
    },
    {
      key: 'uptime',
      icon: <RocketOutlined />,
      tint: 'nc-tint-teal2',
      label: t('pages.index.uptime'),
      value: TimeFormatter.formatSecond(status.uptime),
      foot: `Xray ${status.xray.state} · ${status.appStats.threads} ${t('pages.index.threads')}`,
      border: '#087e82',
    },
  ];

  return (
    <div className="nc-stats nc-stats-4">
      {cards.map((c) => (
        <div key={c.key} className="nc-stat" style={{ borderColor: c.border }}>
          <div className="nc-stat-top">
            <span className={`nc-stat-icon ${c.tint}`}>{c.icon}</span>
            <ThunderboltOutlined className="nc-stat-mini" />
          </div>
          <div className="nc-stat-label">{c.label}</div>
          <div className="nc-stat-value">{c.value}</div>
          <div className="nc-stat-foot">{c.foot}</div>
        </div>
      ))}
    </div>
  );
}

interface OverviewRailProps {
  status: Status;
  panelVersion: string;
}

/** Right-hand rail: server information and donut gauges for each resource. */
export function OverviewRail({ status, panelVersion }: OverviewRailProps) {
  const { t } = useTranslation();

  const info = [
    {
      icon: <ApiOutlined />,
      label: t('pages.index.ipAddresses'),
      value: String(status.publicIP.ipv4),
    },
    {
      icon: <ClockCircleOutlined />,
      label: 'Xray',
      value: status.xray.version || status.xray.state,
    },
    { icon: <RocketOutlined />, label: t('menu.dashboard'), value: `v${panelVersion}` },
    {
      icon: <ThunderboltOutlined />,
      label: t('pages.index.systemHistoryTitle'),
      value: status.loads.map((l) => Number(l).toFixed(2)).join(' · '),
    },
  ];

  const gauges = [
    {
      key: 'cpu',
      icon: <DashboardOutlined />,
      label: t('pages.index.cpu'),
      pct: status.cpu.percent,
      cap: `${CPUFormatter.cpuCoreFormat(status.cpuCores)} / ${status.logicalPro}T`,
      c1: '#0a9fff',
      c2: '#1d6de9',
    },
    {
      key: 'mem',
      icon: <DatabaseOutlined />,
      label: t('pages.index.memory'),
      pct: status.mem.percent,
      cap: `${SizeFormatter.sizeFormat(status.mem.current)} / ${SizeFormatter.sizeFormat(status.mem.total)}`,
      c1: '#7c35f1',
      c2: '#4b31da',
    },
    {
      key: 'swap',
      icon: <SwapOutlined />,
      label: t('pages.index.swap'),
      pct: status.swap.percent,
      cap: `${SizeFormatter.sizeFormat(status.swap.current)} / ${SizeFormatter.sizeFormat(status.swap.total)}`,
      c1: '#00cdb0',
      c2: '#13a9a0',
    },
    {
      key: 'disk',
      icon: <HddOutlined />,
      label: t('pages.index.storage'),
      pct: status.disk.percent,
      cap: `${SizeFormatter.sizeFormat(status.disk.current)} / ${SizeFormatter.sizeFormat(status.disk.total)}`,
      c1: '#13d6b0',
      c2: '#0f827d',
    },
  ];

  const R = 22;
  const C = 2 * Math.PI * R;

  return (
    <aside className="nc-rail">
      <div className="nc-card">
        <div className="nc-card-head">
          <ApiOutlined />
          <span>{t('pages.index.panel')}</span>
        </div>
        <div className="nc-info">
          {info.map((r) => (
            <div key={r.label} className="nc-info-row">
              <span className="nc-info-icon">{r.icon}</span>
              <span className="nc-info-label">{r.label}</span>
              <span className="nc-info-value" dir="ltr">
                {r.value}
              </span>
            </div>
          ))}
        </div>
      </div>

      <div className="nc-card">
        <div className="nc-card-head">
          <DashboardOutlined />
          <span>{t('pages.index.systemHistoryTitle')}</span>
        </div>
        <div className="nc-gauges">
          {gauges.map((g) => {
            const dash = (C * Math.min(100, Math.max(0, g.pct))) / 100;
            return (
              <div key={g.key} className="nc-gauge-row">
                <span className="nc-gauge">
                  <svg viewBox="0 0 54 54" width={54} height={54}>
                    <circle cx="27" cy="27" r={R} fill="none" stroke="#0d315e" strokeWidth={5.5} />
                    <circle
                      cx="27"
                      cy="27"
                      r={R}
                      fill="none"
                      stroke={`url(#gg-${g.key})`}
                      strokeWidth={5.5}
                      strokeLinecap="round"
                      strokeDasharray={`${dash} ${C - dash}`}
                    />
                    <defs>
                      <linearGradient id={`gg-${g.key}`} x1="0" y1="0" x2="1" y2="1">
                        <stop offset="0%" stopColor={g.c1} />
                        <stop offset="100%" stopColor={g.c2} />
                      </linearGradient>
                    </defs>
                  </svg>
                  <b>{g.pct.toFixed(0)}%</b>
                </span>
                <span className="nc-gauge-main">
                  <span className="nc-gauge-line">
                    <span className="nc-gauge-label">
                      {g.icon} {g.label}
                    </span>
                    <span className="nc-gauge-cap" dir="ltr">
                      {g.cap}
                    </span>
                  </span>
                  <span className="nc-bar">
                    <i
                      style={{
                        width: `${Math.min(100, Math.max(0, g.pct))}%`,
                        background: `linear-gradient(90deg, ${g.c2}, ${g.c1})`,
                      }}
                    />
                  </span>
                </span>
              </div>
            );
          })}
        </div>
      </div>

      <div className="nc-card">
        <div className="nc-card-head">
          <SwapOutlined />
          <span>{t('pages.index.overallSpeed')}</span>
        </div>
        <div className="nc-rail-bars">
          <div className="nc-rail-bar">
            <span className="nc-rail-bar-label">
              <ArrowUpOutlined /> {t('pages.index.upload')}
            </span>
            <span className="nc-rail-bar-value">{SizeFormatter.speedFormat(status.netIO.up)}</span>
          </div>
          <div className="nc-rail-bar">
            <span className="nc-rail-bar-label">
              <ArrowDownOutlined /> {t('pages.index.download')}
            </span>
            <span className="nc-rail-bar-value">
              {SizeFormatter.speedFormat(status.netIO.down)}
            </span>
          </div>
          <div className="nc-rail-bar">
            <span className="nc-rail-bar-label">{t('pages.index.received')}</span>
            <span className="nc-rail-bar-value">
              {SizeFormatter.sizeFormat(status.netTraffic.recv)}
            </span>
          </div>
        </div>
      </div>
    </aside>
  );
}
