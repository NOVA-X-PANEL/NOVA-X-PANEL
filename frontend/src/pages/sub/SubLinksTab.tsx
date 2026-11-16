import { useTranslation } from 'react-i18next';
import { Button, QRCode, Tag } from 'antd';
import { CopyOutlined, DownloadOutlined } from '@ant-design/icons';

import SubQrButton from './SubQrButton';

export interface OpenVPNSubData {
  enabled?: boolean;
  name?: string;
  password?: string;
  downloadUrl?: string;
}

interface SubLinksTabProps {
  subUrl: string;
  subJsonUrl: string;
  subClashUrl: string;
  openvpn?: OpenVPNSubData | null;
  onCopy: (value: string) => void;
}

const appendRawView = (url: string) => `${url}${url.includes('?') ? '&' : '?'}view=raw`;

export default function SubLinksTab({ subUrl, subJsonUrl, subClashUrl, openvpn, onCopy }: SubLinksTabProps) {
  const { t } = useTranslation();
  const subLabel = t('pages.settings.subSettings');
  const rows = [
    { kind: 'SUB', color: 'green', url: subUrl, title: subLabel, downloadable: false },
    {
      kind: 'JSON',
      color: 'purple',
      url: subJsonUrl,
      title: `${subLabel} JSON`,
      downloadable: true,
    },
    { kind: 'CLASH', color: 'gold', url: subClashUrl, title: 'Clash / Mihomo', downloadable: true },
    ...(openvpn?.enabled && openvpn.downloadUrl
      ? [{ kind: 'OPENVPN', color: 'cyan', url: openvpn.downloadUrl, title: 'OpenVPN (.ovpn)', downloadable: true }]
      : []),
  ].filter((row) => row.url);

  return (
    <div className="sub-rows">
      {rows.map((row) => (
        <div key={row.kind} className="sub-row">
          <Tag color={row.color} className="sub-row-tag">
            {row.kind}
          </Tag>
          <div className="sub-row-main">
            <a href={row.url} target="_blank" rel="noopener noreferrer" className="sub-row-title">
              {row.title}
            </a>
            <div className="sub-row-url" dir="ltr" title={row.url}>
              {row.url}
            </div>
          </div>
          <div className="sub-row-actions">
            {row.downloadable && (
              <Button
                href={appendRawView(row.url)}
                target="_blank"
                rel="noopener noreferrer"
                icon={<DownloadOutlined />}
                aria-label={t('download')}
                title={t('download')}
              />
            )}
            <Button
              icon={<CopyOutlined />}
              onClick={() => onCopy(row.url)}
              aria-label={t('copy')}
              title={t('copy')}
            />
            <SubQrButton value={row.url} label={row.title} onCopy={onCopy} />
          </div>
        </div>
      ))}
      {openvpn?.enabled && (
        <div className="sub-row" style={{ flexDirection: 'column', alignItems: 'stretch', gap: 12, padding: '16px 20px', background: 'rgba(6, 182, 212, 0.05)', borderColor: 'rgba(6, 182, 212, 0.25)' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: 8 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <Tag color="cyan" style={{ fontWeight: 600 }}>OPENVPN</Tag>
              <span style={{ fontWeight: 600, fontSize: 15 }}>تنظیمات اتصال کلاینت و روتر OpenVPN</span>
            </div>
            {openvpn.downloadUrl && (
              <Button
                type="primary"
                icon={<DownloadOutlined />}
                href={openvpn.downloadUrl}
                target="_blank"
                rel="noopener noreferrer"
                style={{ background: '#0891b2', borderColor: '#0891b2' }}
              >
                دانلود مستقیم فایل کانفیگ (.ovpn)
              </Button>
            )}
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: 10, marginTop: 4 }}>
            {openvpn.name && (
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '8px 12px', background: 'var(--sub-row-bg, rgba(0,0,0,0.03))', borderRadius: 6, border: '1px solid rgba(140, 140, 140, 0.15)' }}>
                <span style={{ fontSize: 13, color: 'var(--text-muted, #888)' }}>نام کاربری (Username):</span>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                  <code style={{ fontSize: 13, fontWeight: 600 }}>{openvpn.name}</code>
                  <Button size="small" type="text" icon={<CopyOutlined />} onClick={() => onCopy(openvpn.name || '')} />
                </div>
              </div>
            )}
            {openvpn.password && (
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '8px 12px', background: 'var(--sub-row-bg, rgba(0,0,0,0.03))', borderRadius: 6, border: '1px solid rgba(140, 140, 140, 0.15)' }}>
                <span style={{ fontSize: 13, color: 'var(--text-muted, #888)' }}>رمز عبور (Password):</span>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                  <code style={{ fontSize: 13, fontWeight: 600 }}>{openvpn.password}</code>
                  <Button size="small" type="text" icon={<CopyOutlined />} onClick={() => onCopy(openvpn.password || '')} />
                </div>
              </div>
            )}
          </div>
          <div className="sub-muted" style={{ fontSize: 12 }}>
            💡 مشخصات فوق برای راه‌اندازی دستی در روترهای میکروتیک، ایسوس و کانفیگ‌های بدون Auth درون‌خطی است. با ایمپورت فایل کانفیگ در نرم‌افزار OpenVPN Connect، اتصال به صورت خودکار برقرار می‌شود.
          </div>
        </div>
      )}
      {subUrl && (
        <div className="sub-qr-card">
          <div className="sub-qr-code">
            <QRCode
              value={subUrl}
              size={112}
              type="svg"
              bordered={false}
              color="#000000"
              bgColor="#ffffff"
            />
          </div>
          <div>
            <div className="sub-qr-title">{t('subscription.scanTitle')}</div>
            <div className="sub-muted">{t('subscription.scanHint')}</div>
          </div>
        </div>
      )}
    </div>
  );
}
