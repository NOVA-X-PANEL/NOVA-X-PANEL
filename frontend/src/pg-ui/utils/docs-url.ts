// @ts-nocheck — vendored from Heimdall-Panel (upstream pg-ui); keep byte-compatible with upstream.
export function getDocsUrl(path = '') {
  const clean = String(path || '').replace(/^\/+/, '');
  return clean ? `https://docs.pasarguard.com/${clean}` : 'https://docs.pasarguard.com/';
}

export default getDocsUrl;
