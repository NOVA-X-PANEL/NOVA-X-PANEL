import { describe, it, expect } from 'vitest';

import { computeTrafficDisplay } from '@/lib/clients/traffic-display';

describe('computeTrafficDisplay', () => {
  const gb = 1024 * 1024 * 1024;

  it('returns 50% for half-used limited quota', () => {
    const d = computeTrafficDisplay(
      { up: 0.25 * gb, down: 0.25 * gb, total: gb, enabled: true, trafficDiff: 0 },
      false,
    );
    expect(d.percent).toBe(50);
    expect(d.isUnlimited).toBe(false);
    expect(d.remaining).toBe(0.5 * gb);
  });

  it('returns 100% bar for unlimited clients', () => {
    const d = computeTrafficDisplay(
      { up: 5 * gb, down: 2 * gb, total: 0, enabled: true, trafficDiff: 0 },
      false,
    );
    expect(d.percent).toBe(100);
    expect(d.isUnlimited).toBe(true);
    expect(d.strokeColor).toBe('#5b7cff');
  });

  it('marks depleted clients with exception status', () => {
    const d = computeTrafficDisplay(
      { up: gb, down: 0, total: gb, enabled: true, trafficDiff: 0 },
      false,
    );
    expect(d.isDepleted).toBe(true);
    expect(d.status).toBe('exception');
    expect(d.percent).toBe(100);
  });

  it('uses gray stroke when client is disabled', () => {
    const d = computeTrafficDisplay(
      { up: 0.5 * gb, down: 0, total: gb, enabled: false, trafficDiff: 0 },
      false,
    );
    expect(d.strokeColor).toBe('#bcbcbc');
    expect(d.status).toBeUndefined();
  });

  // The three usage levels use the Neon Console hues rather than antd's
  // success/warning/danger tokens, so a full bar reads as an alert without
  // clashing with the surrounding surface.
  it('uses the near-limit colour when the remaining budget is inside the diff', () => {
    const diff = 0.1 * gb;
    const d = computeTrafficDisplay(
      { up: 0.95 * gb, down: 0, total: gb, enabled: true, trafficDiff: diff },
      false,
    );
    expect(d.strokeColor).toBe('#f0a020');
  });

  it('uses the low-usage colour while there is room to spare', () => {
    const d = computeTrafficDisplay(
      { up: 0.2 * gb, down: 0, total: gb, enabled: true, trafficDiff: 0 },
      false,
    );
    expect(d.strokeColor).toBe('#00cfa8');
  });

  it('uses the over-limit colour once the budget is spent', () => {
    const d = computeTrafficDisplay(
      { up: gb, down: 0, total: gb, enabled: true, trafficDiff: 0 },
      false,
    );
    expect(d.strokeColor).toBe('#e0348a');
  });
});
