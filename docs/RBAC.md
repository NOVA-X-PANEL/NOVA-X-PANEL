# RBAC — Admin Roles & Permissions (NOVA X PANEL)

This document describes the role-based access-control core added to NOVA X PANEL.
It is derived from the design used by the upstream Heimdall fork of 3x-ui.

## Concepts

| Concept | Table | Notes |
|---|---|---|
| Panel account | `users` | `username`, `password`, `role_id`, `status` |
| Role | `admin_roles` | four JSON documents (see below) |
| Client | `clients` | `owner_admin_id` binds a client to its owning account |
| Client group | `client_groups` | referenced by the role `access` document |

Each panel account is bound to exactly one role. The **first** account (lowest
`id`) is seeded as the `owner` role; every other account defaults to
`administrator`.

## The four role documents

All four are stored as JSON text so new keys need no migration.

### 1. `permissions` — resource → action → value

A value is `true`, `false`, or `{ "scope": 0 | 1 | 2 }`.

| scope | meaning |
|---|---|
| `0` | none — the action is denied |
| `1` | own — only records owned by the acting admin |
| `2` | all — every record (owner role only in practice) |

Resources: `inbounds`, `users` (clients), `groups`, `nodes`, `admins`,
`admin_roles`, `outbounds`, `routing`, `settings`, `cores`, `hosts`, `system`.

Route permissions accept both spellings (`view`/`read`, `update_core`/`updateCore`,
`roles`/`admin_roles`, `clients`/`users`) — the alias tables live in
`internal/web/controller/admin_security.go` and `internal/web/service/client_rbac.go`.

### 2. `limits` — bounds on clients an admin may create

`max_users`, `data_limit_min|max`, `expire_days_min|max`,
`download_mbps_min|max`, `upload_mbps_min|max`, `min|maxOnHoldTimeoutDays`.
Empty = inherit; `0` = disabled.

### 3. `features` — behavioural flags

`blockLimitedAdmins`, `disconnectUsersWhenLimited`, `disconnectUsersWhenDisabled`,
`can_use_reset_strategy`, `can_use_next_plan`.

### 4. `access` — group / inbound restriction

`allowAllGroups` + `allowedGroups`, and `allowAllInbounds` + `allowed_inbound_ids`.

## Built-in roles

| slug | owner_role | built_in | summary |
|---|---|---|---|
| `owner` | ✅ | ✅ | full access; read-only role |
| `administrator` | ❌ | ✅ | almost every permission; clients scoped to own |
| `operator` | ❌ | ✅ | minimal; manages only its own clients |

## Enforcement

Middleware (`internal/web/controller/admin_security.go`):

- `loginActiveAdminRole` — requires a browser session (API tokens rejected),
  an `active` account, and resolves the role.
- `requireOwnerAdminMiddleware` — owner role only.
- `requireAdminPermission(section, permission)` — single permission check.
- `requireAnyPanelPermission(...)` — passes if any requirement is met.

Only the **owner** role can see clients across admins. Every other role is
capped at `own` scope even when its permission says `all`.

## API

### Admins — `/panel/api/admins/*`

`GET current`, `GET list`, `GET stats`, `GET get/:id`,
`POST add`, `POST update/:id`, `POST del/:id`,
`POST enable/:id`, `POST disable/:id`.

The owner account cannot be deleted, disabled, or demoted; you cannot delete or
disable your own account.

### Roles — `/panel/api/admin-roles/*`

`GET list`, `GET get/:id`, `POST add`, `POST update/:id`,
`POST duplicate/:id`, `POST del/:id`.

The owner role is read-only; built-in roles cannot be deleted; a role assigned to
an admin cannot be deleted; a role slug is derived from its name.

## Source layout

```
internal/database/model/admin_role.go        role model + default presets
internal/database/db.go                      model registration + seedAdminRBAC()
internal/database/model/model.go             User.role_id/status, ClientRecord.owner_admin_id
internal/web/service/client_rbac.go          client access scope + role readers
internal/web/service/panel/admin.go          admin account service
internal/web/service/panel/admin_role.go     role service
internal/web/controller/admin_security.go    permission middleware
internal/web/controller/admins.go            admin API
internal/web/controller/admin_roles.go       role API
```

## Front-end

Two pages ship with the panel:

| Route | Page | Key actions |
|---|---|---|
| `/panel/admins` | Administrators | list, create, edit, enable/disable, delete |
| `/panel/admin-roles` | Admin Roles | list, create, edit, duplicate, delete |

The role editor exposes four tabs — **Permissions**, **Limits**, **Features**,
**Access**. Scoped client actions use a `none / own / all` selector; plain
actions use a checkbox. Access can be narrowed to specific client groups and
inbound IDs.

Both pages live in the sidebar under *Admins* and *Admin Roles*. They reuse the
existing query layer (`src/api/queries/useAdmins.ts`) and the shared permission
helper `src/lib/rbac.ts` (`isOwner`, `hasPermission`).

### Front-end files

```
frontend/src/pages/admins/AdminsPage.tsx              page shell, stats, delete confirm
frontend/src/pages/admins/AdminList.tsx               list card (toolbar + table)
frontend/src/pages/admins/AdminFormModal.tsx          create/edit + password reset
frontend/src/pages/admins/AdminList.css
frontend/src/pages/admins/AdminsPage.css
frontend/src/pages/admin-roles/AdminRolesPage.tsx     page shell, stats
frontend/src/pages/admin-roles/AdminRoleList.tsx      list card (toolbar + table)
frontend/src/pages/admin-roles/AdminRoleModal.tsx     four-tab role editor
frontend/src/pages/admin-roles/AdminRoleList.css
frontend/src/pages/admin-roles/AdminRolesPage.css
frontend/src/api/queries/useAdmins.ts                 queries + mutations
frontend/src/lib/rbac.ts                              permission groups + helpers
```

### Page shell

Both pages use the panel's shared page shell so they match every other page:

- wrapped in `<ConfigProvider theme={antdThemeConfig}>`
- `Layout className={pageClass}` where `pageClass` is `<name>-page` plus
  `is-dark` / `is-ultra`
- `content-shell` → `content-area`, with the standard `summary-card` (four
  `Statistic` tiles at `xs=12 sm=12 md=6`) followed by the list card
- `<Spin spinning={!fetched}>` → `<Result status="error">` on failure
- `Card size="small" hoverable` + a space-between `.toolbar` for the list, and a
  `Dropdown` row-action menu, matching `NodeList` / `HostList`
- a `modal.confirm` before destructive actions

`admins-page` and `admin-roles-page` are registered in
`frontend/src/styles/page-shell.css` and `frontend/src/styles/page-cards.css`,
which is what gives them the shared background, content padding, card radius and
hover shadows.

## Status

Backend and front-end both implemented. The version file is `1.1.1`.

## 1.1.1 — a permissioned page must be able to finish loading

Granting only `inbounds.view` showed the Inbounds page but it never finished
loading (a spinner that survived a refresh). Two independent causes:

1. **Structural reads were gated.** `setting/all` (the browser-safe settings view
   the shell and every page read) and `setting/defaultSettings` /
   `factoryDefaults` / `getDefaultJsonConfig` (the default templates) sat behind
   `settings.view*`. A role without settings access got a 403 for data the page
   cannot render without. They now use `requirePanelAccount()`: any active panel
   account may read them, no resource permission attached.
2. **Supplementary queries blocked the page.** `InboundsPage` gates its spinner on
   `inbounds.fetched && hosts.fetched`, and `hosts/list` needs `hosts.view`. A
   403 left `hostsError` set, so the page rendered the error state forever.

`useHostsQuery` and `useNodesQuery` are now permission-aware: they read the
current account, skip the request entirely when the role cannot read that
resource, and report `fetched: true` with no error, so a page that merely *shows*
hosts/nodes alongside its own content still renders. `permitted` is exposed for
callers that want to hide the related UI.

## Front-end: ported from Heimdall (1.0.6)

The admin and role screens are no longer hand-built with antd. They are the
**actual Heimdall pg-ui** components, ported verbatim, so the layout and
interaction are identical to upstream:

```
frontend/src/pg-ui/**            vendored Heimdall UI (shadcn/ui + Tailwind v4)
frontend/src/app/providers/theme-provider.tsx   theme shim over this panel's theme
frontend/src/pages/admins/AdminsPage.tsx        shell wrapper -> pg-ui page
frontend/src/pages/admin-roles/AdminRolesPage.tsx
```

How the port works:

- **Tailwind v4** via `@tailwindcss/vite`, configured from
  `src/pg-ui/tailwind.config.js` and `src/pg-ui/styles/pasarguard.css`.
  Preflight is deliberately **not** imported: this panel is a mixed codebase and
  Tailwind's global resets would silently restyle every antd page once an admin
  screen was opened. Only `theme` + `utilities` are layered in.
- **shadcn/ui + Radix** primitives ship inside `pg-ui/components/ui`.
- **Theme**: pg-ui reads `@/app/providers/theme-provider`, which is a thin shim
  over this panel's own theme context, so `.dark` on `<body>` keeps driving both
  antd and Tailwind.
- **App shell**: Heimdall's `PanelLayout` supplies the sidebar; this panel's
  pages render their own, so the two wrappers above do it for these screens and
  hand pg-ui an unpadded full-width column (its `PageHeader` owns the padding).
- **Vendored code is exempt** from oxlint/oxfmt (`ignorePatterns`) and carries
  `// @ts-nocheck`, because upstream does not build against this repo's stricter
  `noUnusedLocals` / `verbatimModuleSyntax` settings.
- **API compatibility**: pg-ui's `service/api.ts` already normalises both
  camelCase and snake_case, and it calls the same `HttpUtil` + route names this
  panel exposes, so no adapter was needed beyond four new endpoints:
  `admins/resetUsage/:id`, `admins/users/{disableActive,activateDisabled,removeAll}/:id`.

### Heimdall-parity notes (1.0.5)

The two RBAC pages now mirror the upstream Heimdall panel's information
architecture:

- **Admins** — four statistic tiles (Total / Active / Disabled / **Limited**);
  the table shows administrator, role, status, owned client count and a usage
  bar; the row menu offers edit, reset password, enable/disable and delete.
  Each account carries an optional **data limit** (`data_limit`, bytes; 0 =
  unlimited) and an aggregated `used_bytes`, so an account over quota is
  reported as *limited*.
- **Admin Roles** — the editor is a four-tab dialog:
  - **Permissions**: one collapsible block per resource with an
    `enabled/total` counter and *Select all* / *Clear*; each action is a row
    with a human-readable label, a *Scoped* marker, and either a
    `None / Own / All` select (scoped client actions) or a switch.
  - **Limits**: `max_users` plus the byte/day/throughput bounds, empty =
    inherit, 0 = disabled.
  - **Feature flags**: one bordered row per flag with a title, a hint and a
    switch.
  - **Access**: allow-all switches for client groups and inbounds, with
    multi-select pickers when restricted.

Resource and action labels come from `pages.adminRoles.resources.*` and
`pages.adminRoles.actionLabels.*`, falling back to `humanizeKey()` (the same
behaviour as Heimdall). Note: `description` is a reserved go-i18n key, so the
per-flag help text is stored under `hint`.


## 1.0.9 — actually enforcing the role

Until 1.0.9 the role was only stored: the permission middleware was wired to the
admin and role endpoints alone, so any signed-in account reached the whole panel
regardless of its role. Two things were missing.

### Route enforcement

`requirePanelPermission(resource, action)` now guards the panel routes, mirroring
Heimdall's mapping:

| resource | guarded routes |
|---|---|
| `inbounds` | list / options (`viewSimple`) / get / add / update / delete / resetTraffic / import |
| `groups` | list / emails / create / rename / delete / bulkAdd / bulkRemove |
| `hosts` | list / get / byInbound / tags / add / update / delete / setEnable / reorder / bulk |
| `nodes` | list / get / webCert / add / update / delete / setEnable / test / probe (`reconnect`) / updatePanel (`updateCore`) / history (`viewStatistics`) / mtls |
| `settings` | all / defaultSettings / factoryDefaults / update / restartPanel / apiTokens-ish / testSmtp / testTgBot / testDiscord |
| `cores` | xray read + every mutating xray/outbound-subs route |

`POST /panel/api/setting/all` accepts `settings.view` **or** `settings.viewGeneral`
because the sidebar depends on it.

### Client scoping

A role whose `permissions.users.*` carry `scope: 1` ("own") now sees only its own
clients:

- `ClientController.clientScope()` resolves the acting account's scope; API-token
  callers keep full access because `enforceTokenScope` already narrowed them and
  node sync has no panel account.
- `list`, `listPaged`, `get`, `update`, `delete`, `resetTraffic/:email` and every
  bulk endpoint filter through `FilterClientEmailsForScope` /
  `RequireClientForScopeByEmail`, so a scoped admin cannot reach another admin's
  client even by guessing its email.
- `create` refuses callers without `users.create`.
- created clients are stamped with `owner_admin_id` (`AssignOwnerAdmin`), which is
  what makes "own" mode resolvable.

### Front-end

- `RouteGuard` (Heimdall's component) wraps the router: a role that cannot read a
  page is redirected to `firstAllowedRoute`.
- the sidebar hides entries the role cannot open, so the menu matches what the
  guard will allow.
