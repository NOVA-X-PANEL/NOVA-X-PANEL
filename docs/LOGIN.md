# Login screen

The sign-in page (`frontend/src/pages/login/`) is a bespoke design rather than a
themed antd form. It is a single CSS scope, `.nova-login`, so none of it leaks
into the panel's antd pages.

## Layout

Two columns on desktop, stacked under 1024px:

| Column | Contents |
|---|---|
| Hero | brand mark + `NOVA X PANEL`, the English tagline, the Persian slogan, three feature blocks (speed / secure / global), and the landscape |
| Auth | glass card with the mark, title, Persian subtitle, username, password (with reveal toggle), optional 2FA field, gradient submit, remember-me, forgot link, `یا` divider, Telegram button, then the footer |

## Design tokens

All colours, radii and the font stack live on `.nova-login` as custom properties
(`--nx-bg`, `--nx-blue`, `--nx-cyan`, `--nx-line`, `--nx-radius`, `--nx-font`), so
retuning the screen means editing one block.

## Assets

- `NovaLogo.tsx` — angular hexagonal crest with an `N` cut from it. Gradient ids
  are namespaced per instance (`idSuffix`) because the mark renders three times.
- `NovaLandscape.tsx` — one SVG holding the whole depth stack: sky gradient, the
  glowing planet on the horizon, cloud bands, three mountain ridges and
  snow-rim highlights. `preserveAspectRatio="xMidYMax slice"` keeps the horizon
  pinned to the bottom edge as the panel narrows.
- `src/assets/fonts/Vazirmatn-{Regular,Medium,Bold}.woff2` (~50 KB each) are
  bundled rather than pulled from a CDN, so an offline panel still renders
  Persian correctly. Referenced from `LoginPage.css` via `../../assets/fonts/` so
  Vite fingerprints and rebases them under the configured base path.

## RTL

The root sets `dir` from the active i18next language (Persian/Arabic/Hebrew →
`rtl`). Layout rules use logical properties (`border-inline-end`,
`inset-inline-end`) so the same markup mirrors instead of needing a second
stylesheet. `ConfigProvider.direction` is set too, because antd renders the
language menu in a body portal outside `.nova-login`.

## Pre-auth probe

`POST /getLoginOptions` returns:

```json
{ "twoFactorEnable": false, "telegramEnabled": false, "telegramLoginSupported": false }
```

The screen uses it to decide whether to show the 2FA field and whether the
Telegram button is usable. It falls back to the older `POST /getTwoFactorEnable`
if the panel predates the endpoint.

**Telegram sign-in is not implemented.** The button is rendered but disabled and
tooltipped accordingly, rather than being a silent no-op.
