import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { InputNumber, message } from 'antd';
import {
  CloudDownloadOutlined,
  CloudUploadOutlined,
  DownOutlined,
  PlusOutlined,
  ReloadOutlined,
  SettingOutlined,
  TeamOutlined,
} from '@ant-design/icons';

import NeonLandscape from '@/components/NeonLandscape';
import {
  type ClientDefaults,
  readClientDefaults,
  writeClientDefaults,
} from '@/lib/clients/default-limits';

/**
 * Page chrome for the Clients screen, matching the Neon Console reference: a hero
 * banner with two headline metrics, and a full-width action bar above the table.
 *
 * The action bar replaced a right-hand rail. The rail took a third of the width,
 * which left the clients table scrolling horizontally for no good reason, so the
 * actions now sit in a row above it and the table keeps the whole column. The
 * client defaults are a collapsible inside the same card rather than a separate
 * dialog: they are two numbers, and a section that opens in place costs one click
 * instead of three.
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

interface ClientsActionsProps {
  onAdd: () => void;
  onImport: () => void;
  onExport: () => void;
  onResetTraffic: () => void;
}

/** Full-width action bar: the four client actions, plus the default-limits editor. */
export function ClientsActions({ onAdd, onImport, onExport, onResetTraffic }: ClientsActionsProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [open, setOpen] = useState(false);
  // Read once, lazily: the values live in this browser's storage and reading them
  // in an effect would render the empty state first and then swap it.
  const [defaults, setDefaults] = useState<ClientDefaults>(() => readClientDefaults());

  const saveDefaults = () => {
    const next = writeClientDefaults(defaults);
    setDefaults(next);
    messageApi.success(t('pages.clients.defaultLimitsSaved'));
  };

  const summary =
    defaults.limitIp > 0 || defaults.limitHwid > 0
      ? `IP ${defaults.limitIp} · HWID ${defaults.limitHwid}`
      : '—';

  const actions = [
    {
      key: 'add',
      icon: <PlusOutlined />,
      label: t('pages.clients.addClients'),
      onClick: onAdd,
      cls: 'is-primary',
    },
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
      cls: 'is-danger',
    },
  ];

  return (
    <div className="nc-actionbar">
      {messageContextHolder}
      <div className="nc-card nc-actionbar-card">
        <div className="nc-actionbar-row">
          {actions.map((a) => (
            <button
              key={a.key}
              type="button"
              className={`nc-action-flat${a.cls ? ` ${a.cls}` : ''}`}
              onClick={a.onClick}
            >
              <span className="nc-action-flat-icon">{a.icon}</span>
              <span>{a.label}</span>
            </button>
          ))}

          <button
            type="button"
            className={`nc-action-flat nc-defaults-trigger${open ? ' is-open' : ''}`}
            aria-expanded={open}
            onClick={() => setOpen((v) => !v)}
          >
            <span className="nc-action-flat-icon">
              <SettingOutlined />
            </span>
            <span>{t('pages.clients.defaultLimits')}</span>
            <span className="nc-defaults-summary">{summary}</span>
            <DownOutlined className="nc-defaults-chevron" />
          </button>
        </div>
      </div>

      {open && (
        <div className="nc-collapse is-open">
          <p className="nc-collapse-desc">{t('pages.clients.defaultLimitsHint')}</p>
          <div className="nc-collapse-fields">
            <label className="nc-field-row">
              <span>{t('pages.clients.limitIp')}</span>
              <InputNumber
                min={0}
                value={defaults.limitIp}
                onChange={(v) => setDefaults((d) => ({ ...d, limitIp: Number(v) || 0 }))}
              />
            </label>
            <label className="nc-field-row">
              <span>{t('pages.clients.limitHwid')}</span>
              <InputNumber
                min={0}
                value={defaults.limitHwid}
                onChange={(v) => setDefaults((d) => ({ ...d, limitHwid: Number(v) || 0 }))}
              />
            </label>
          </div>
          <div className="nc-collapse-foot">
            <button type="button" className="nc-save" onClick={saveDefaults}>
              {t('save')}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
