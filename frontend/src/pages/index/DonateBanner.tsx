import type { FC } from 'react';
import { useTranslation } from 'react-i18next';
import { ExportOutlined } from '@ant-design/icons';
import './DonateBanner.css';

const DONATE_URL = 'https://reymit.ir/nova-x';

export const DonateBanner: FC = () => {
  const { t } = useTranslation();

  return (
    <div
      className="ov-donate-banner"
      role="region"
      aria-label={t('pages.index.donateTitle', { defaultValue: 'حمایت مالی از پروژه نوا' })}
    >
      <div className="ov-donate-glow-line" />
      <div className="ov-donate-content">
        <div className="ov-donate-icon-box" aria-hidden="true">
          <svg
            className="ov-donate-icon"
            viewBox="0 0 24 24"
            width="26"
            height="26"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <path d="M18 8h1a4 4 0 0 1 0 8h-1" />
            <path d="M2 8h16v9a4 4 0 0 1-4 4H6a4 4 0 0 1-4-4V8z" />
            <line x1="6" y1="2" x2="6" y2="4" />
            <line x1="10" y1="2" x2="10" y2="4" />
            <line x1="14" y1="2" x2="14" y2="4" />
          </svg>
        </div>

        <div className="ov-donate-text">
          <div className="ov-donate-header">
            <h3 className="ov-donate-title">
              {t('pages.index.donateTitle', { defaultValue: 'حمایت مالی از پروژه نوا' })}
            </h3>
            <span className="ov-donate-tag">reymit.ir/nova-x</span>
          </div>
          <p className="ov-donate-desc">
            {t('pages.index.donateDesc', {
              defaultValue:
                'با دونیت و حمایت مالی در پلتفرم ریمیت به توسعه رایگان و پایداری این پروژه کمک کنید.',
            })}
          </p>
        </div>
      </div>

      <a href={DONATE_URL} target="_blank" rel="noopener noreferrer" className="ov-donate-btn">
        <span>{t('pages.index.donateBtn', { defaultValue: 'حمایت مالی (دونیت)' })}</span>
        <ExportOutlined className="ov-donate-btn-icon" />
      </a>
    </div>
  );
};
