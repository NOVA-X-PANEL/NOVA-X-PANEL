import { useTranslation } from 'react-i18next';

import NovaLogo from '../login/NovaLogo';

interface OverviewHeroProps {
  version: string;
}

export default function OverviewHero({ version }: OverviewHeroProps) {
  const { t } = useTranslation();

  return (
    <div className="ov-hero">
      <span className="ov-hero-mark">
        <NovaLogo size={46} idSuffix="ovhero" />
      </span>

      <div className="ov-hero-text">
        <div className="ov-hero-title">NOVA X PANEL</div>
        <div className="ov-hero-sub">{t('menu.dashboard')}</div>
      </div>

      <span className="ov-hero-version">v{version}</span>
    </div>
  );
}
