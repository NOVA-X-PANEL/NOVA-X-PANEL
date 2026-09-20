export interface AdminRoleDoc {
  id: number;
  name: string;
  slug: string;
  builtIn: boolean;
  ownerRole: boolean;
  permissions: Record<string, Record<string, unknown>>;
  limits: Record<string, unknown>;
  features: Record<string, unknown>;
  access: Record<string, unknown>;
  adminCount: number;
  createdAt: number;
  updatedAt: number;
}

export interface AdminAccount {
  id: number;
  username: string;
  roleId: number;
  roleName: string;
  roleSlug: string;
  ownerRole: boolean;
  status: string;
  isSelf: boolean;
  createdAt: number;
  updatedAt: number;
}

export interface AdminStats {
  totalAdmins: number;
  activeAdmins: number;
  disabledAdmins: number;
}

export interface CurrentAdmin {
  id: number;
  username: string;
  status: string;
  roleId: number;
  isOwner: boolean;
  permissions: Record<string, Record<string, unknown>>;
  limits: Record<string, unknown>;
  features: Record<string, unknown>;
  access: Record<string, unknown>;
  role: {
    id: number;
    name: string;
    slug: string;
    ownerRole: boolean;
    permissions: Record<string, Record<string, unknown>>;
  };
}

/** Permission scope: 0 = none, 1 = own, 2 = all. */
export type PermissionScope = 0 | 1 | 2;

export type PermissionValue = boolean | { scope: PermissionScope };

export interface PermissionActionDef {
  resource: string;
  action: string;
  /** True when the action supports a none/own/all scope selector. */
  scoped?: boolean;
}

export interface PermissionGroupDef {
  labelKey: string;
  actions: PermissionActionDef[];
}

/** Resources an action can target, grouped for the role editor. */
export const PERMISSION_GROUPS: PermissionGroupDef[] = [
  {
    labelKey: 'inbounds',
    actions: [
      { resource: 'inbounds', action: 'read' },
      { resource: 'inbounds', action: 'read_simple' },
      { resource: 'inbounds', action: 'create' },
      { resource: 'inbounds', action: 'update' },
      { resource: 'inbounds', action: 'delete' },
      { resource: 'inbounds', action: 'reset_usage' },
    ],
  },
  {
    labelKey: 'clients',
    actions: [
      { resource: 'users', action: 'read', scoped: true },
      { resource: 'users', action: 'read_simple', scoped: true },
      { resource: 'users', action: 'create' },
      { resource: 'users', action: 'update', scoped: true },
      { resource: 'users', action: 'delete', scoped: true },
      { resource: 'users', action: 'reset_usage', scoped: true },
      { resource: 'users', action: 'revoke_sub', scoped: true },
      { resource: 'users', action: 'set_owner', scoped: true },
      { resource: 'users', action: 'activate_next_plan', scoped: true },
      { resource: 'users', action: 'admin_filter' },
    ],
  },
  {
    labelKey: 'groups',
    actions: [
      { resource: 'groups', action: 'read' },
      { resource: 'groups', action: 'read_simple' },
      { resource: 'groups', action: 'create' },
      { resource: 'groups', action: 'update' },
      { resource: 'groups', action: 'delete' },
    ],
  },
  {
    labelKey: 'nodes',
    actions: [
      { resource: 'nodes', action: 'read' },
      { resource: 'nodes', action: 'read_simple' },
      { resource: 'nodes', action: 'create' },
      { resource: 'nodes', action: 'update' },
      { resource: 'nodes', action: 'delete' },
      { resource: 'nodes', action: 'reconnect' },
      { resource: 'nodes', action: 'update_core' },
      { resource: 'nodes', action: 'stats' },
      { resource: 'nodes', action: 'logs' },
    ],
  },
  {
    labelKey: 'admins',
    actions: [
      { resource: 'admins', action: 'read' },
      { resource: 'admins', action: 'read_simple' },
      { resource: 'admins', action: 'create' },
      { resource: 'admins', action: 'update' },
      { resource: 'admins', action: 'delete' },
      { resource: 'admins', action: 'reset_usage' },
    ],
  },
  {
    labelKey: 'roles',
    actions: [
      { resource: 'admin_roles', action: 'read' },
      { resource: 'admin_roles', action: 'read_simple' },
      { resource: 'admin_roles', action: 'create' },
      { resource: 'admin_roles', action: 'update' },
      { resource: 'admin_roles', action: 'delete' },
    ],
  },
  {
    labelKey: 'outbounds',
    actions: [
      { resource: 'outbounds', action: 'read' },
      { resource: 'outbounds', action: 'create' },
      { resource: 'outbounds', action: 'update' },
      { resource: 'outbounds', action: 'delete' },
    ],
  },
  {
    labelKey: 'routing',
    actions: [
      { resource: 'routing', action: 'read' },
      { resource: 'routing', action: 'create' },
      { resource: 'routing', action: 'update' },
      { resource: 'routing', action: 'delete' },
    ],
  },
  {
    labelKey: 'settings',
    actions: [
      { resource: 'settings', action: 'read' },
      { resource: 'settings', action: 'read_general' },
      { resource: 'settings', action: 'update' },
    ],
  },
  {
    labelKey: 'xrayConfigs',
    actions: [
      { resource: 'cores', action: 'read' },
      { resource: 'cores', action: 'read_simple' },
      { resource: 'cores', action: 'create' },
      { resource: 'cores', action: 'update' },
      { resource: 'cores', action: 'delete' },
      { resource: 'hosts', action: 'read' },
      { resource: 'hosts', action: 'create' },
      { resource: 'hosts', action: 'update' },
    ],
  },
  {
    labelKey: 'overview',
    actions: [{ resource: 'system', action: 'read' }],
  },
];

export const LIMIT_KEYS = [
  'max_users',
  'data_limit_min',
  'data_limit_max',
  'expire_days_min',
  'expire_days_max',
  'download_mbps_min',
  'download_mbps_max',
  'upload_mbps_min',
  'upload_mbps_max',
] as const;

export const FEATURE_KEYS = [
  'blockLimitedAdmins',
  'disconnectUsersWhenLimited',
  'disconnectUsersWhenDisabled',
  'can_use_reset_strategy',
  'can_use_next_plan',
] as const;

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}

function parseMaybeJSON(value: unknown): unknown {
  if (typeof value !== 'string') return value;
  const trimmed = value.trim();
  if (!trimmed || (!trimmed.startsWith('{') && !trimmed.startsWith('['))) return value;
  try {
    return JSON.parse(trimmed) as unknown;
  } catch {
    return value;
  }
}

/** Resolves a role's permission map from any of the supported shapes. */
export function permissionsOf(admin: unknown): Record<string, Record<string, unknown>> {
  const root = asRecord(admin);
  const role = asRecord(root.role);
  const candidates = [
    role.permissions,
    role.permissions_json,
    root.permissions,
    root.permissions_json,
    root.rolePermissions,
  ];
  for (const candidate of candidates) {
    const parsed = parseMaybeJSON(candidate);
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, Record<string, unknown>>;
    }
  }
  return {};
}

export function isOwner(admin: unknown): boolean {
  const root = asRecord(admin);
  const role = asRecord(root.role);
  return Boolean(
    role.is_owner ||
    role.owner_role ||
    role.ownerRole ||
    role.slug === 'owner' ||
    root.isOwner ||
    root.ownerRole ||
    root.slug === 'owner',
  );
}

function permissionValueAllowed(value: unknown): boolean {
  if (value === true) return true;
  if (value === false || value == null) return false;
  if (typeof value === 'number') return value !== 0;
  if (typeof value === 'string') {
    return ['true', 'yes', '1', 'own', 'all', '2'].includes(value.trim().toLowerCase());
  }
  if (typeof value === 'object') {
    return permissionValueAllowed((value as { scope?: unknown }).scope);
  }
  return false;
}

/** Reports whether the signed-in admin holds resource.action. */
export function hasPermission(admin: unknown, resource: string, action: string): boolean {
  if (isOwner(admin)) return true;
  const permissions = permissionsOf(admin);
  const section = permissions[resource];
  if (!section || typeof section !== 'object') return false;
  return permissionValueAllowed(section[action]);
}
