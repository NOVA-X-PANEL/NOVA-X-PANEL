import { useTranslation } from 'react-i18next';
import { Button, Tag } from 'antd';
import { CopyOutlined, DownloadOutlined, LockOutlined, UserOutlined, WifiOutlined } from '@ant-design/icons';
import QRCode from 'qrcode.react';

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

const neonStyle = {
  borderColor: 'rgba(8, 145, 178, 0.5)',
  boxShadow: '0 0 10px rgba(8, 145, 178, 0.2)',
};

export default function SubLinksTab({ subUrl, subJsonUrl, subClashUrl, openvpn, onCopy }: SubLinksTabProps) {
  const { t } = useTranslation();

  return (
    <div className="sub-rows">
      <Tag color="green" className="sub-row-tag">
        SUB
      </Tag>
      <div className="sub-row-main">
        <a href={subUrl} target="_blank" rel="noopener noreferrer" className="sub-row-title">
          {t('pages.settings.subSettings')}
        </a>
        <div className="sub-row-url" dir="ltr" title={subUrl}>
          {subUrl}
        </div>
      </div>
      <div className="sub-row-actions">
        <Button
          icon={<CopyOutlined />}
          onClick={() => onCopy(subUrl)}
          aria-label={t('copy')}
          title={t('copy')}
        />
        <SubQrButton value={subUrl} label={t('pages.settings.subSettings')} onCopy={onCopy} />
      </div>

      {subJsonUrl && (
        <>
          <Tag color="purple" className="sub-row-tag">
            JSON
          </Tag>
          <div className="sub-row-main">
            <a href={subJsonUrl} target="_blank" rel="noopener noreferrer" className="sub-row-title">
              {t('pages.settings.subSettings')} JSON
            </a>
            <div className="sub-row-url" dir="ltr" title={subJsonUrl}>
              {subJsonUrl}
            </div>
          </div>
          <div className="sub-row-actions">
            <Button
              href={appendRawView(subJsonUrl)}
              target="_blank"
              rel="noopener noreferrer"
              icon={<DownloadOutlined />}
              aria-label={t('download')}
              title={t('download')}
            />
            <Button
              icon={<CopyOutlined />}
              onClick={() => onCopy(subJsonUrl)}
              aria-label={t('copy')}
              title={t('copy')}
            />
            <SubQrButton value={subJsonUrl} label={`${t('pages.settings.subSettings')} JSON`} onCopy={onCopy} />
          </div>
        </>
      )}

      {subClashUrl && (
        <>
          <Tag color="gold" className="sub-row-tag">
            CLASH
          </Tag>
          <div className="sub-row-main">
            <a href={subClashUrl} target="_blank" rel="noopener noreferrer" className="sub-row-title">
              Clash / Mihomo
            </a>
            <div className="sub-row-url" dir="ltr" title={subClashUrl}>
              {subClashUrl}
            </div>
          </div>
          <div className="sub-row-actions">
            <Button
              href={appendRawView(subClashUrl)}
              target="_blank"
              rel="noopener noreferrer"
              icon={<DownloadOutlined />}
              aria-label={t('download')}
              title={t('download')}
            />
            <Button
              icon={<CopyOutlined />}
              onClick={() => onCopy(subClashUrl)}
              aria-label={t('copy')}
              title={t('copy')}
            />
            <SubQrButton value={subClashUrl} label="Clash / Mihomo" onCopy={onCopy} />
          </div>
        </>
      )}

      {openvpn?.enabled && (
        <div
          className="sub-openvpn-cyber-card"
          style={{
            background: 'linear-gradient(145deg, rgba(8, 145, 178, 0.06) 0%, rgba(8, 145, 178, 0.12) 100%)',
            border: '1px solid rgba(8, 145, 178, 0.4)',
            borderRadius: '16px',
            padding: '28px',
            marginTop: '16px',
            boxShadow: '0 0 25px rgba(8, 145, 178, 0.12), inset 0 1px 0 rgba(8, 145, 178, 0.1)',
            position: 'relative',
            overflow: 'hidden',
          }}
        >
          <div
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              right: 0,
              height: '2px',
              background: 'linear-gradient(90deg, transparent, #0891b2, #00e5ff, #0891b2, transparent)',
              animation: 'scanline 3s linear infinite',
            }}
          />
          <style jsx>{`
            @keyframes scanline {
              0% { opacity: 0.3; }
              50% { opacity: 1; }
              100% { opacity: 0.3; }
            }
          `}</style>

          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '20px', flexWrap: 'wrap', gap: '12px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
              <div
                style={{
                  width: '44px',
                  height: '44px',
                  borderRadius: '12px',
                  background: 'linear-gradient(135deg, #0891b2 0%, #00e5ff 100%)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  boxShadow: '0 0 20px rgba(8, 145, 178, 0.4)',
                }}
              >
                <LockOutlined style={{ color: '#000', fontSize: '20px', fontWeight: 900 }} />
              </div>
              <div>
                <h3 style={{ margin: 0, color: '#0891b2', fontSize: '20px', fontWeight: 700, letterSpacing: '0.5px' }}>
                  OpenVPN Subscription Center
                </h3>
                <p style={{ margin: '4px 0 0', color: 'rgba(8, 145, 178, 0.7)', fontSize: '13px', fontFamily: 'monospace' }}>
                  PROTOCOL: TLSv1.3 | CIPHER: AES-256-GCM | AUTH: SHA256
                </p>
              </div>
            </div>

            {openvpn.downloadUrl && (
              <Button
                type="primary"
                icon={<DownloadOutlined />}
                href={openvpn.downloadUrl}
                target="_blank"
                rel="noopener noreferrer"
                size="large"
                style={{
                  background: 'linear-gradient(135deg, #0891b2 0%, #00e5ff 100%)',
                  border: 'none',
                  borderRadius: '10px',
                  fontWeight: 600,
                  padding: '10px 20px',
                  boxShadow: '0 0 20px rgba(8, 145, 178, 0.4)',
                  transition: 'all 0.3s ease',
                }}
              >
                دانلود مستقیم .ovpn
              </Button>
            )}
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: '16px', marginBottom: '16px' }}>
            {openvpn.name && (
              <div
                style={{
                  ...neonStyle,
                  background: 'rgba(0, 0, 0, 0.2)',
                  borderRadius: '12px',
                  padding: '16px 20px',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '8px',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: 'rgba(8, 145, 178, 0.8)', fontSize: '12px', textTransform: 'uppercase', letterSpacing: '1px' }}>
                  <UserOutlined style={{ fontSize: '14px' }} />
                  نام کاربری (Username)
                </div>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '8px' }}>
                  <code style={{ fontSize: '16px', fontWeight: 700, color: '#00e5ff', fontFamily: 'monospace', background: 'rgba(0, 229, 255, 0.1)', padding: '8px 12px', borderRadius: '8px', border: '1px solid rgba(8, 145, 178, 0.3)' }}>
                    {openvpn.name}
                  </code>
                  <Button
                    size="small"
                    type="text"
                    icon={<CopyOutlined />}
                    onClick={() => onCopy(openvpn.name || '')}
                    style={{ color: '#0891b2' }}
                  />
                </div>
              </div>
            )}

            {openvpn.password && (
              <div
                style={{
                  ...neonStyle,
                  background: 'rgba(0, 0, 0, 0.2)',
                  borderRadius: '12px',
                  padding: '16px 20px',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '8px',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: 'rgba(8, 145, 178, 0.8)', fontSize: '12px', textTransform: 'uppercase', letterSpacing: '1px' }}>
                  <LockOutlined style={{ fontSize: '14px' }} />
                  رمز عبور (Password)
                </div>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '8px' }}>
                  <code style={{ fontSize: '16px', fontWeight: 700, color: '#00e5ff', fontFamily: 'monospace', background: 'rgba(0, 229, 255, 0.1)', padding: '8px 12px', borderRadius: '8px', border: '1px solid rgba(8, 145, 178, 0.3)' }}>
                    {openvpn.password}
                  </code>
                  <Button
                    size="small"
                    type="text"
                    icon={<CopyOutlined />}
                    onClick={() => onCopy(openvpn.password || '')}
                    style={{ color: '#0891b2' }}
                  />
                </div>
              </div>
            )}
          </div>

          {openvpn.downloadUrl && (
            <div
              style={{
                ...neonStyle,
                background: 'rgba(0, 0, 0, 0.2)',
                borderRadius: '12px',
                padding: '16px 20px',
                display: 'flex',
                flexDirection: 'column',
                gap: '12px',
                marginBottom: '16px',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: 'rgba(8, 145, 178, 0.8)', fontSize: '12px', textTransform: 'uppercase', letterSpacing: '1px' }}>
                <WifiOutlined style={{ fontSize: '14px' }} />
                لینک اشتراک OpenVPN (برای کلاینت‌های پشتیبانی‌کننده)
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '12px', flexWrap: 'wrap' }}>
                <div style={{ flex: 1, minWidth: 200 }}>
                  <code style={{ fontSize: '13px', color: 'rgba(8, 145, 178, 0.9)', fontFamily: 'monospace', background: 'rgba(0, 0, 0, 0.3)', padding: '10px 12px', borderRadius: '8px', border: '1px solid rgba(8, 145, 178, 0.2)', display: 'block', wordBreak: 'break-all' }}>
                    {openvpn.downloadUrl}
                  </code>
                </div>
                <div style={{ display: 'flex', gap: '8px' }}>
                  <Button
                    icon={<CopyOutlined />}
                    onClick={() => onCopy(openvpn.downloadUrl)}
                    style={{ color: '#0891b2' }}
                  />
                  <Button
                    href={openvpn.downloadUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    icon={<DownloadOutlined />}
                    style={{ color: '#00e5ff', background: 'rgba(8, 145, 178, 0.1)', borderColor: 'rgba(8, 145, 178, 0.3)' }}
                  />
                  <SubQrButton value={openvpn.downloadUrl} label="OpenVPN Subscription" onCopy={onCopy} />
                </div>
              </div>
            </div>
          )}

          <div
            style={{
              paddingTop: '16px',
              borderTop: '1px solid rgba(8, 145, 178, 0.15)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              flexWrap: 'wrap',
              gap: '12px',
              fontSize: '12px',
              color: 'rgba(8, 145, 178, 0.6)',
              fontFamily: 'monospace',
            }}
          >
            <span>💡 برای روترها: از نام کاربری و رمز بالا در تنظیمات OpenVPN Client استفاده کنید</span>
            <span>برای موبایل/PC: فایل .ovpn را دانلود و در OpenVPN Connect ایمپورت کنید</span>
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