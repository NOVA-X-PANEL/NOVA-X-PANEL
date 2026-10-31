// @ts-nocheck
import React, { useState, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  Cell,
  CartesianGrid,
} from 'recharts';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/pg-ui/components/ui/card';
import { formatBytes } from '@/pg-ui/utils/formatByte';
import useDirDetection from '@/pg-ui/hooks/use-dir-detection';
import { cn } from '@/pg-ui/lib/utils';
import type { AdminDetails } from '@/pg-ui/service/api';
import {
  BarChart3,
  TrendingUp,
  HardDrive,
  Users,
  AlertTriangle,
  Zap,
} from 'lucide-react';

interface AdminUsageChartProps {
  admins?: AdminDetails[];
}

export default function AdminUsageChart({ admins = [] }: AdminUsageChartProps) {
  const { t } = useTranslation();
  const dir = useDirDetection();
  const [metricMode, setMetricMode] = useState<'current' | 'lifetime' | 'quota'>('current');

  const { chartData, totalCurrent, totalLifetime, topConsumer, quotaExceededCount } = useMemo(() => {
    if (!admins || admins.length === 0) {
      return {
        chartData: [],
        totalCurrent: '0 B',
        totalLifetime: '0 B',
        topConsumer: null,
        quotaExceededCount: 0,
      };
    }

    let sumCurrent = 0;
    let sumLifetime = 0;
    let maxCurrent = -1;
    let topAdmin: (typeof admins)[0] | null = null;
    let exceeded = 0;

    const data = admins.map((adm) => {
      const rawUsed = Number(adm.used_traffic) || 0;
      const rawLifetime = Number(adm.lifetime_used_traffic) || 0;
      const rawLimit = Number(adm.data_limit) || 0;

      sumCurrent += rawUsed;
      sumLifetime += rawLifetime;

      if (rawUsed > maxCurrent) {
        maxCurrent = rawUsed;
        topAdmin = adm;
      }

      const isExceeded = rawLimit > 0 && rawUsed >= rawLimit;
      if (isExceeded) {
        exceeded++;
      }

      const percent = rawLimit > 0 ? Math.min(100, Math.round((rawUsed / rawLimit) * 100)) : 0;
      const usedGB = parseFloat((rawUsed / (1024 ** 3)).toFixed(3));
      const lifetimeGB = parseFloat((rawLifetime / (1024 ** 3)).toFixed(3));
      const limitGB = rawLimit > 0 ? parseFloat((rawLimit / (1024 ** 3)).toFixed(3)) : null;

      return {
        id: adm.id,
        username: adm.username || `Admin-${adm.id}`,
        roleName: adm.role?.name || (adm.role_id === 1 ? 'Owner' : 'Admin'),
        rawUsed,
        rawLifetime,
        rawLimit,
        usedGB,
        lifetimeGB,
        limitGB,
        usedStr: formatBytes(rawUsed),
        lifetimeStr: formatBytes(rawLifetime),
        limitStr: rawLimit > 0 ? formatBytes(rawLimit) : t('unlimited', { defaultValue: 'نامحدود' }),
        percent,
        isExceeded,
        totalUsers: adm.total_users || 0,
        status: adm.status || (adm.is_disabled ? 'disabled' : 'active'),
      };
    });

    // Sort by current usage descending for clear ranking
    data.sort((a, b) => (metricMode === 'lifetime' ? b.rawLifetime - a.rawLifetime : b.rawUsed - a.rawUsed));

    return {
      chartData: data,
      totalCurrent: formatBytes(sumCurrent),
      totalLifetime: formatBytes(sumLifetime),
      topConsumer: topAdmin,
      quotaExceededCount: exceeded,
    };
  }, [admins, metricMode, t]);

  if (!admins || admins.length === 0) {
    return null;
  }

  const CustomTooltip = ({ active, payload }: any) => {
    if (!active || !payload || !payload.length) return null;
    const item = payload[0].payload;

    return (
      <div
        className="rounded-lg p-3 text-xs shadow-2xl backdrop-blur-xl border border-cyan-500/30"
        style={{
          background: 'rgba(15, 23, 42, 0.92)',
          color: '#f8fafc',
          minWidth: 200,
          direction: dir,
        }}
      >
        <div className="flex items-center justify-between gap-2 border-b border-slate-700/60 pb-2 mb-2">
          <div className="flex items-center gap-1.5 font-bold text-sm text-cyan-400">
            <Zap className="h-4 w-4 text-cyan-400" />
            <span>{item.username}</span>
          </div>
          <span className="text-[10px] px-1.5 py-0.5 rounded bg-slate-800 text-slate-300 border border-slate-700">
            {item.roleName}
          </span>
        </div>

        <div className="space-y-1.5">
          <div className="flex items-center justify-between">
            <span className="text-slate-400">{t('admins.currentUsage', { defaultValue: 'مصرف دوره جاری:' })}</span>
            <span className="font-semibold text-emerald-400 font-mono">{item.usedStr}</span>
          </div>

          <div className="flex items-center justify-between">
            <span className="text-slate-400">{t('admins.lifetimeUsage', { defaultValue: 'مصرف کل تاریخچه:' })}</span>
            <span className="font-semibold text-indigo-400 font-mono">{item.lifetimeStr}</span>
          </div>

          <div className="flex items-center justify-between">
            <span className="text-slate-400">{t('admins.dataLimit', { defaultValue: 'سقف مجاز سهمیه:' })}</span>
            <span
              className={cn(
                'font-semibold font-mono',
                item.isExceeded ? 'text-red-400' : item.rawLimit > 0 ? 'text-amber-400' : 'text-slate-400'
              )}
            >
              {item.limitStr}
            </span>
          </div>

          {item.rawLimit > 0 && (
            <div className="pt-1 border-t border-slate-800">
              <div className="flex justify-between text-[11px] mb-1">
                <span className="text-slate-400">درصد مصرف:</span>
                <span className={cn('font-bold', item.isExceeded ? 'text-red-400' : 'text-cyan-400')}>
                  {item.percent}%
                </span>
              </div>
              <div className="w-full bg-slate-800 h-1.5 rounded-full overflow-hidden">
                <div
                  className={cn(
                    'h-full rounded-full transition-all duration-300',
                    item.isExceeded
                      ? 'bg-red-500'
                      : item.percent > 80
                      ? 'bg-amber-500'
                      : 'bg-gradient-to-r from-cyan-500 to-emerald-500'
                  )}
                  style={{ width: `${item.percent}%` }}
                />
              </div>
            </div>
          )}

          <div className="flex items-center justify-between pt-1 text-[11px] text-slate-400">
            <span>{t('admins.totalClients', { defaultValue: 'تعداد کاربران:' })}</span>
            <span className="text-slate-200 font-mono">{item.totalUsers}</span>
          </div>
        </div>
      </div>
    );
  };

  return (
    <Card className="w-full overflow-hidden border border-primary/20 bg-card/60 backdrop-blur-md shadow-xl transition-all duration-300 mb-3">
      <CardHeader className="pb-3 pt-4 px-4 sm:px-6">
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
          <div className="flex items-center gap-2.5">
            <div className="p-2 rounded-lg bg-cyan-500/10 border border-cyan-500/20 text-cyan-400">
              <BarChart3 className="h-5 w-5" />
            </div>
            <div>
              <CardTitle className="text-base sm:text-lg font-bold flex items-center gap-2">
                <span>{t('admins.usageAnalyticsTitle', { defaultValue: 'نمودار تحلیلی مصرف ترافیک مدیران' })}</span>
                <span className="text-[11px] px-2 py-0.5 rounded-full bg-cyan-500/10 text-cyan-400 border border-cyan-500/20 font-normal">
                  Live RBAC
                </span>
              </CardTitle>
              <CardDescription className="text-xs text-muted-foreground mt-0.5">
                {t(
                  'admins.usageAnalyticsDesc',
                  { defaultValue: 'پایش و مقایسه برخط حجم ترافیک مصرفی، سقف‌های تعیین‌شده و نرخ استفاده هر ادمین' }
                )}
              </CardDescription>
            </div>
          </div>

          {/* Metric Mode Toggle */}
          <div className="flex items-center rounded-lg bg-muted/60 p-1 border border-border/50 text-xs self-stretch sm:self-auto justify-center">
            <button
              type="button"
              onClick={() => setMetricMode('current')}
              className={cn(
                'px-3 py-1 rounded-md font-medium transition-all duration-200',
                metricMode === 'current'
                  ? 'bg-primary text-primary-foreground shadow-sm font-semibold'
                  : 'text-muted-foreground hover:text-foreground'
              )}
            >
              {t('admins.modeCurrent', { defaultValue: 'دوره جاری' })}
            </button>
            <button
              type="button"
              onClick={() => setMetricMode('lifetime')}
              className={cn(
                'px-3 py-1 rounded-md font-medium transition-all duration-200',
                metricMode === 'lifetime'
                  ? 'bg-primary text-primary-foreground shadow-sm font-semibold'
                  : 'text-muted-foreground hover:text-foreground'
              )}
            >
              {t('admins.modeLifetime', { defaultValue: 'کل تاریخچه' })}
            </button>
          </div>
        </div>

        {/* Metric Summary Badges */}
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-2.5 pt-3 mt-1 border-t border-border/40">
          <div className="rounded-md p-2.5 bg-background/50 border border-border/40 flex items-center gap-2.5">
            <div className="p-1.5 rounded bg-emerald-500/10 text-emerald-400">
              <TrendingUp className="h-4 w-4" />
            </div>
            <div className="min-w-0">
              <div className="text-[10px] text-muted-foreground truncate">
                {t('admins.totalCurrentUsed', { defaultValue: 'مجموع مصرف جاری' })}
              </div>
              <div className="font-bold text-sm text-emerald-400 font-mono truncate">{totalCurrent}</div>
            </div>
          </div>

          <div className="rounded-md p-2.5 bg-background/50 border border-border/40 flex items-center gap-2.5">
            <div className="p-1.5 rounded bg-indigo-500/10 text-indigo-400">
              <HardDrive className="h-4 w-4" />
            </div>
            <div className="min-w-0">
              <div className="text-[10px] text-muted-foreground truncate">
                {t('admins.totalLifetimeUsed', { defaultValue: 'مجموع مصرف کل' })}
              </div>
              <div className="font-bold text-sm text-indigo-400 font-mono truncate">{totalLifetime}</div>
            </div>
          </div>

          <div className="rounded-md p-2.5 bg-background/50 border border-border/40 flex items-center gap-2.5">
            <div className="p-1.5 rounded bg-cyan-500/10 text-cyan-400">
              <Users className="h-4 w-4" />
            </div>
            <div className="min-w-0">
              <div className="text-[10px] text-muted-foreground truncate">
                {t('admins.topConsumer', { defaultValue: 'پرمصرف‌ترین مدیر' })}
              </div>
              <div className="font-bold text-sm text-cyan-400 truncate">
                {topConsumer ? topConsumer.username : '-'}
              </div>
            </div>
          </div>

          <div className="rounded-md p-2.5 bg-background/50 border border-border/40 flex items-center gap-2.5">
            <div
              className={cn(
                'p-1.5 rounded',
                quotaExceededCount > 0 ? 'bg-red-500/10 text-red-400' : 'bg-slate-500/10 text-slate-400'
              )}
            >
              <AlertTriangle className="h-4 w-4" />
            </div>
            <div className="min-w-0">
              <div className="text-[10px] text-muted-foreground truncate">
                {t('admins.exceededLimit', { defaultValue: 'تکمیل سقف مجاز' })}
              </div>
              <div
                className={cn(
                  'font-bold text-sm font-mono truncate',
                  quotaExceededCount > 0 ? 'text-red-400' : 'text-muted-foreground'
                )}
              >
                {quotaExceededCount}{' '}
                <span className="text-[10px] font-normal">{t('admins.adminUnit', { defaultValue: 'ادمین' })}</span>
              </div>
            </div>
          </div>
        </div>
      </CardHeader>

      <CardContent className="px-2 sm:px-6 pb-4 pt-1">
        <div className="w-full h-[260px] sm:h-[280px]">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart
              data={chartData}
              margin={{ top: 15, right: 10, left: 0, bottom: 25 }}
              barGap={4}
            >
              <CartesianGrid strokeDasharray="3 3" stroke="#334155" opacity={0.3} vertical={false} />
              <XAxis
                dataKey="username"
                tick={{ fill: '#94a3b8', fontSize: 11 }}
                axisLine={{ stroke: '#475569', opacity: 0.4 }}
                tickLine={false}
                interval={0}
                angle={-25}
                textAnchor="end"
                height={40}
              />
              <YAxis
                tick={{ fill: '#94a3b8', fontSize: 10 }}
                axisLine={false}
                tickLine={false}
                unit=" GB"
                width={50}
              />
              <Tooltip content={<CustomTooltip />} cursor={{ fill: 'rgba(255, 255, 255, 0.04)' }} />
              <Bar
                dataKey={metricMode === 'lifetime' ? 'lifetimeGB' : 'usedGB'}
                radius={[6, 6, 0, 0]}
                maxBarSize={52}
                animationDuration={800}
              >
                {chartData.map((entry, index) => {
                  let fillColor = '#00f2fe';
                  if (entry.isExceeded) {
                    fillColor = '#ef4444'; // Red if exceeded quota
                  } else if (entry.percent > 80) {
                    fillColor = '#f59e0b'; // Amber if near limit
                  } else if (metricMode === 'lifetime') {
                    fillColor = index === 0 ? '#818cf8' : '#6366f1'; // Indigo for lifetime
                  } else {
                    fillColor = index === 0 ? '#00f2fe' : '#0ea5e9'; // Cyan for current
                  }

                  return <Cell key={`cell-${entry.id}`} fill={fillColor} fillOpacity={0.85} />;
                })}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        </div>
      </CardContent>
    </Card>
  );
}
