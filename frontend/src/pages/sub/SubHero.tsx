import { useEffect, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Progress, Tag, theme } from 'antd';

import { IntlUtil } from '@/utils';
import type { CalendarKind } from '@/utils';
import { daysUntil, remainingTimeLabel, resolveSubStatus, usagePercent } from './subPageModel';
import type { SubStatus } from './subPageModel';

interface SubHeroProps {
  enabled: boolean;
  usedByte: number;
  totalByte: number;
  expireMs: number;
  lastOnlineMs: number;
  download: string;
  upload: string;
  used: string;
  total: string;
  remained: string;
  datepicker: CalendarKind;
  lang: string;
}

const STATUS_TAGS: Record<SubStatus, { color: string; label: string }> = {
  active: { color: 'green', label: 'subscription.active' },
  unlimited: { color: 'purple', label: 'subscription.unlimited' },
  expired: { color: 'red', label: 'subscription.expired' },
  depleted: { color: 'red', label: 'subscription.depleted' },
  disabled: { color: 'red', label: 'subscription.inactive' },
};

// FormatTraffic renders "37.60GB"; the amount and unit are sized apart.
function splitSize(label: string): [string, string] {
  const match = /^([\d.,]+)\s*(\D*)$/.exec(label.trim());
  return match ? [match[1], match[2]] : [label, ''];
}

export default function SubHero({
  enabled,
  usedByte,
  totalByte,
  expireMs,
  lastOnlineMs,
  download,
  upload,
  used,
  total,
  remained,
  datepicker,
  lang,
}: SubHeroProps) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, []);

  const hasQuota = totalByte > 0;
  const currentStatus = resolveSubStatus({ enabled, usedByte, totalByte, expireMs }, now);
  const currentDaysLeft = daysUntil(expireMs, now);
  const healthy = currentStatus === 'active' || currentStatus === 'unlimited';
  const pct = usagePercent(usedByte, totalByte);
  const ringColor =
    !healthy || pct >= 90 ? token.colorError : pct >= 75 ? token.colorWarning : token.colorPrimary;
  const timeRingColor = !healthy ? token.colorError : token.colorPrimary;
  const [amount, unit] = splitSize(hasQuota ? remained : used);
  const formatDate = (ms: number) => IntlUtil.formatDate(ms, datepicker, lang);
  const statusTag = STATUS_TAGS[currentStatus];

  const stats: { key: string; label: string; value: ReactNode }[] = [
    { key: 'days', label: t('subscription.daysLeft'), value: currentDaysLeft ?? '∞' },
    {
      key: 'expiry',
      label: t('subscription.expiry'),
      value: expireMs > 0 ? formatDate(expireMs) : t('subscription.noExpiry'),
    },
    {
      key: 'status',
      label: t('subscription.status'),
      value: <Tag color={statusTag.color}>{t(statusTag.label)}</Tag>,
    },
    { key: 'down', label: t('subscription.downloaded'), value: <bdi>{download}</bdi> },
    { key: 'up', label: t('subscription.uploaded'), value: <bdi>{upload}</bdi> },
    { key: 'total', label: t('subscription.totalQuota'), value: <bdi>{total}</bdi> },
    {
      key: 'lastOnline',
      label: t('lastOnline'),
      value: lastOnlineMs > 0 ? formatDate(lastOnlineMs) : '-',
    },
  ];

  return (
    <section className={healthy ? 'sub-hero' : 'sub-hero is-alert'}>
      <div className="sub-rings">
        <Progress
          type="circle"
          className="sub-ring"
          percent={pct}
          status="normal"
          size={156}
          strokeColor={ringColor}
          format={() => (
            <span className="sub-ring-center">
              <span className="sub-ring-value">{hasQuota ? `${pct.toFixed(1)}%` : '∞'}</span>
              <span className="sub-ring-label">
                {hasQuota ? t('usage') : t('subscription.unlimited')}
              </span>
            </span>
          )}
        />
        <Progress
          type="circle"
          className="sub-ring sub-time-ring"
          percent={expireMs > 0 && healthy ? 100 : 0}
          status="normal"
          size={156}
          strokeColor={timeRingColor}
          format={() => (
            <span className="sub-ring-center">
              <span className="sub-ring-value sub-time-value">
                {remainingTimeLabel(expireMs, now)}
              </span>
              <span className="sub-ring-label">{t('subscription.remainingTime')}</span>
            </span>
          )}
        />
      </div>
      <div className="sub-hero-summary">
        <div className="sub-label">{hasQuota ? t('remained') : t('usage')}</div>
        <bdi className="sub-big">
          <span className="sub-big-num">{amount}</span>
          {unit && <span className="sub-big-unit">{unit}</span>}
        </bdi>
        <div className="sub-muted">
          {hasQuota ? t('subscription.ofTotal', { total }) : t('subscription.unlimited')}
        </div>
        <dl className="sub-stats">
          {stats.map((stat) => (
            <div key={stat.key} className="sub-stat">
              <dt className="sub-label">{stat.label}</dt>
              <dd className="sub-stat-value">{stat.value}</dd>
            </div>
          ))}
        </dl>
      </div>
    </section>
  );
}
