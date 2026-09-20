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

## Status

Backend core implemented and building. Front-end pages (`/panel/admins`,
`/panel/admin-roles`) are the next stage; the API is fully usable today.
