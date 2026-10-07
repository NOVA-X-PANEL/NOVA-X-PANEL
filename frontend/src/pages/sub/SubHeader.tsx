import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Menu, Popover, Space, Tag } from 'antd';
import {
  MoonFilled,
  MoonOutlined,
  SunOutlined,
  TranslationOutlined,
  WifiOutlined,
} from '@ant-design/icons';

import { LanguageManager } from '@/utils';
import { pauseAnimationsUntilLeave, useTheme } from '@/hooks/useTheme';

interface SubHeaderProps {
  title: string;
  sId: string;
  email: string;
  lang: string;
  onLangChange: (lang: string) => void;
}

export default function SubHeader({ title, sId, email, lang, onLangChange }: SubHeaderProps) {
  const { t } = useTranslation();
  const { isDark, isUltra, toggleTheme, toggleUltra } = useTheme();

  const cycleTheme = () => {
    pauseAnimationsUntilLeave('sub-theme-cycle');
    if (!isDark) {
      toggleTheme();
      if (isUltra) toggleUltra();
    } else if (!isUltra) {
      toggleUltra();
    } else {
      toggleUltra();
      toggleTheme();
    }
  };

  const langMenuItems = useMemo(
    () =>
      (LanguageManager.supportedLanguages as { value: string; name: string; icon: string }[]).map(
        (l) => ({
          key: l.value,
          label: (
            <Space size={8}>
              <span aria-hidden="true">{l.icon}</span>
              <span>{l.name}</span>
            </Space>
          ),
        }),
      ),
    [],
  );

  const themeIcon = !isDark ? <SunOutlined /> : !isUltra ? <MoonOutlined /> : <MoonFilled />;
  const initial = Array.from(title)[0]?.toUpperCase();

  return (
    <header className="sub-header">
      <div className="sub-brand">
        <span className="sub-brand-mark" aria-hidden="true">
          {initial ?? <WifiOutlined />}
        </span>
        <div className="sub-brand-text">
          <div className="sub-brand-title" dir="auto" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span>{title || t('subscription.title')}</span>
            <Tag color="purple" style={{ margin: 0, fontSize: '10px', fontWeight: 800, borderRadius: '6px', background: 'rgba(113, 64, 255, 0.25)', borderColor: '#7140ff', color: '#c4b5fd' }}>
              PRO TUNNEL
            </Tag>
          </div>
          <div className="sub-brand-id">
            <bdi>{email ? `${sId} - ${email}` : sId}</bdi>
          </div>
        </div>
      </div>
      <div className="sub-toolbar">
        <Button
          id="sub-theme-cycle"
          shape="circle"
          size="large"
          className="toolbar-btn"
          aria-label={t('menu.theme')}
          title={t('menu.theme')}
          icon={themeIcon}
          onClick={cycleTheme}
        />
        <Popover
          rootClassName={isDark ? 'dark' : 'light'}
          placement="bottomRight"
          trigger="click"
          styles={{ content: { padding: 4 } }}
          content={
            <Menu
              mode="vertical"
              selectable
              selectedKeys={[lang]}
              items={langMenuItems}
              onClick={({ key }) => onLangChange(key)}
              style={{ border: 'none', minWidth: 160 }}
            />
          }
        >
          <Button
            shape="circle"
            size="large"
            className="toolbar-btn"
            aria-label={t('pages.settings.language')}
            icon={<TranslationOutlined />}
          />
        </Popover>
      </div>
    </header>
  );
}
