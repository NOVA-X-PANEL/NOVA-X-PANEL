import { useMemo } from 'react';
import type { ReactNode } from 'react';
import { Card, theme } from 'antd';

import { RadialGauge, Sparkline } from '@/components/viz';
import { USAGE_WARN_PERCENT } from '@/models/status';
import { mean, peak } from './useOverviewHistory';
import { NOVA_BLUE, NOVA_VIOLET } from './novaTheme';

interface VitalTileProps {
  icon: ReactNode;
  label: string;
  percent: number;
  statusColor: string;
  detail: string;
  footLeft: string;
  footRight: string;
  data: number[];
  isMobile: boolean;
}

export default function VitalTile({
  icon,
  label,
  percent,
  statusColor,
  detail,
  footLeft,
  footRight,
  data,
  isMobile,
}: VitalTileProps) {
  const { token } = theme.useToken();
  const meanColor = token.colorTextTertiary;

  const referenceLines = useMemo(
    () => (data.length > 1 ? [{ y: mean(data), dash: '3 4', color: meanColor }] : []),
    [data, meanColor],
  );

  // The Nova violet→blue reads as "nominal"; once a resource runs warm the ring
  // takes the panel's own warning/critical colour so the signal stays honest.
  const stressed = percent >= USAGE_WARN_PERCENT;
  const from = stressed ? statusColor : NOVA_VIOLET;
  const to = stressed ? statusColor : NOVA_BLUE;

  return (
    <Card hoverable className="ov-tile" styles={{ body: { padding: 0 } }}>
      <div className="ov-tile-head">
        <span className="ov-tile-icon">{icon}</span>
        <span className="ov-kicker">{label}</span>
      </div>

      <div className="ov-tile-gauge">
        <RadialGauge
          value={percent}
          size={isMobile ? 132 : 152}
          thickness={isMobile ? 9 : 11}
          from={from}
          to={to}
          ariaLabel={`${label} ${percent.toFixed(1)}%`}
        >
          <span className="ov-gauge-value">
            <span className="ov-gauge-number">{percent.toFixed(1)}</span>
            <span className="ov-gauge-unit">%</span>
          </span>
        </RadialGauge>
      </div>

      <div className="ov-tile-detail">{detail}</div>

      <div className="ov-tile-foot">
        <span>{footLeft}</span>
        <span>{footRight}</span>
      </div>

      <div className="ov-tile-chart">
        <Sparkline
          data={data}
          height={isMobile ? 44 : 54}
          strokeWidth={1.5}
          fillOpacity={0.3}
          showGrid={false}
          showMarker={false}
          valueMax={peak(data) > 0 ? null : 100}
          stroke={statusColor}
          referenceLines={referenceLines}
          yFormatter={(v) => `${v.toFixed(0)}%`}
          name1={label}
        />
      </div>
    </Card>
  );
}
