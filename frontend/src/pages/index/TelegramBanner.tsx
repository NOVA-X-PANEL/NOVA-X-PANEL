import type { FC } from 'react';
import { useTranslation } from 'react-i18next';
import { ExportOutlined } from '@ant-design/icons';
import './TelegramBanner.css';

const TELEGRAM_URL = 'https://t.me/NOVA_X_PANELL';

export const TelegramBanner: FC = () => {
  const { t } = useTranslation();

  return (
    <div className="ov-tg-banner" role="region" aria-label={t('pages.index.telegramChannel')}>
      <div className="ov-tg-glow-line" />
      <div className="ov-tg-content">
        <div className="ov-tg-icon-box" aria-hidden="true">
          <svg className="ov-tg-icon" viewBox="0 0 24 24" width="28" height="28" fill="none">
            <path
              d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm4.64 6.8-1.7 8.01c-.13.57-.47.71-.95.44l-2.6-1.92-1.25 1.21c-.14.14-.26.26-.53.26l.19-2.66 4.84-4.37c.21-.19-.05-.29-.32-.11L8.27 13.5l-2.58-.81c-.56-.17-.57-.56.12-.83l10.08-3.88c.47-.17.88.11.75.82z"
              fill="currentColor"
            />
          </svg>
        </div>

        <div className="ov-tg-text">
          <div className="ov-tg-header">
            <h3 className="ov-tg-title">{t('pages.index.telegramChannel')}</h3>
            <span className="ov-tg-tag">@NOVA_X_PANELL</span>
          </div>
          <p className="ov-tg-desc">{t('pages.index.telegramChannelDesc')}</p>
        </div>
      </div>

      <a href={TELEGRAM_URL} target="_blank" rel="noopener noreferrer" className="ov-tg-btn">
        <span>{t('pages.index.telegramChannelBtn')}</span>
        <ExportOutlined className="ov-tg-btn-icon" />
      </a>
    </div>
  );
};
