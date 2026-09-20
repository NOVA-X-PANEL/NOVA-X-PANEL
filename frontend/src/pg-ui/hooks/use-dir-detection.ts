// @ts-nocheck — vendored from Heimdall-Panel (upstream pg-ui); keep byte-compatible with upstream.
import { useTranslation } from 'react-i18next';

const useDirDetection = () => {
  const { i18n } = useTranslation()
  return i18n.dir()
}

export default useDirDetection
