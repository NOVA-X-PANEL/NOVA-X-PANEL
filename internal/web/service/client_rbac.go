package service

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

// RBAC (NOVA X PANEL) — client access scoping.
//
// A role's `permissions.users.*` values carry a scope (0/1/2 = none/own/all)
// and its `access` document lists the allowed client groups and inbound ids.
// These helpers turn that into a ClientAccessScope which is applied to every
// client query so a non-owner admin can only ever see and touch their own
// clients, inside the groups/inbounds the role allows.

// ClientAccessMode is the ownership dimension of a scope.
type ClientAccessMode string

const (
	ClientAccessAll  ClientAccessMode = "all"
	ClientAccessOwn  ClientAccessMode = "own"
	ClientAccessNone ClientAccessMode = "none"
)

// ClientAccessScope is the resolved permission scope for one admin + action.
type ClientAccessScope struct {
	AdminID           int
	Mode              ClientAccessMode
	RestrictGroups    bool
	AllowAllGroups    bool
	AllowedGroups     []string
	RestrictInbounds  bool
	AllowAllInbounds  bool
	AllowedInboundIDs []int
}

func normalizeAllowedClientGroups(groups []string) []string {
	if len(groups) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(groups))
	out := make([]string, 0, len(groups))
	for _, group := range groups {
		key := strings.ToLower(strings.TrimSpace(group))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func normalizeAllowedInboundIDs(ids []int) []int {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(ids))
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func normalizeClientAccessScope(scope ClientAccessScope) ClientAccessScope {
	if scope.Mode == "" {
		scope.Mode = ClientAccessAll
	}
	if scope.Mode == ClientAccessOwn && scope.AdminID <= 0 {
		scope.Mode = ClientAccessNone
	}
	if scope.RestrictGroups {
		scope.AllowedGroups = normalizeAllowedClientGroups(scope.AllowedGroups)
		if scope.AllowAllGroups {
			scope.AllowedGroups = nil
		}
	}
	if scope.RestrictInbounds {
		scope.AllowedInboundIDs = normalizeAllowedInboundIDs(scope.AllowedInboundIDs)
		if scope.AllowAllInbounds {
			scope.AllowedInboundIDs = nil
		}
	}
	return scope
}

// clientGroupAllowed reports whether the scope permits a client in `group`.
func clientGroupAllowed(scope ClientAccessScope, group string) bool {
	scope = normalizeClientAccessScope(scope)
	if !scope.RestrictGroups || scope.AllowAllGroups {
		return true
	}
	if len(scope.AllowedGroups) == 0 {
		return false
	}
	key := strings.ToLower(strings.TrimSpace(group))
	for _, allowed := range scope.AllowedGroups {
		if key == allowed {
			return true
		}
	}
	return false
}

// applyClientGroupAccessScope narrows a query to the scope's allowed groups.
func applyClientGroupAccessScope(db *gorm.DB, scope ClientAccessScope) *gorm.DB {
	scope = normalizeClientAccessScope(scope)
	if !scope.RestrictGroups || scope.AllowAllGroups {
		return db
	}
	if len(scope.AllowedGroups) == 0 {
		return db.Where("1 = 0")
	}
	return db.Where("LOWER(group_name) IN ?", scope.AllowedGroups)
}

// applyClientInboundAccessScope narrows a client query to the clients attached to
// the scope's allowed inbounds.
//
// The inbound side of a role's access document used to be enforced only where an
// operation names an inbound explicitly (ClientInboundsAllowedForScope), so a role
// limited to one inbound still saw every client on the panel in the clients list
// and could read any of them by email. The restriction is part of the scope, so it
// belongs here, where the scope is already applied.
//
// A client attached to no inbound at all cannot be "on an allowed inbound", so it
// is excluded. That is the safe reading: the role said which inbounds it may see,
// and a client that is on none of them is not one of them.
func applyClientInboundAccessScope(db *gorm.DB, scope ClientAccessScope) *gorm.DB {
	scope = normalizeClientAccessScope(scope)
	if !scope.RestrictInbounds || scope.AllowAllInbounds {
		return db
	}
	if len(scope.AllowedInboundIDs) == 0 {
		// Restricted to an empty set: nothing is visible. Matching the empty-state
		// behaviour of the group restriction, which also denies rather than allows.
		return db.Where("1 = 0")
	}
	return db.Where(
		// An inline subquery rather than a nested *gorm.DB: building the inner
		// query from the same session makes gorm share one Statement between the
		// two, and rendering it then recurses until the stack overflows. The
		// statement is spelled out so the two are independent.
		"id IN (SELECT client_id FROM "+model.ClientInbound{}.TableName()+" WHERE inbound_id IN ?)",
		scope.AllowedInboundIDs,
	)
}

// ScopeRestrictsClients reports whether a scope narrows what a caller may see.
//
// Exported because a caller can use it to skip a lookup entirely: a scope that
// restricts nothing has nothing to deny, so there is no point consulting the
// database to confirm it. The check stays authoritative where it matters — a
// restricting scope still denies out-of-scope clients — and the common
// unrestricted path (an owner, an internal caller, a unit test) stops paying for
// a query whose answer cannot be "no".
func ScopeRestrictsClients(scope ClientAccessScope) bool {
	return clientScopeRestricts(scope)
}

// clientScopeRestricts reports whether the scope narrows a client query at all.
// A scope that allows every group, every inbound and every owner leaves the query
// untouched, which lets callers skip the subquery entirely.
func clientScopeRestricts(scope ClientAccessScope) bool {
	scope = normalizeClientAccessScope(scope)
	if scope.Mode != ClientAccessAll {
		return true
	}
	if scope.RestrictGroups && !scope.AllowAllGroups {
		return true
	}
	if scope.RestrictInbounds && !scope.AllowAllInbounds {
		return true
	}
	return false
}

// applyClientAccessScope narrows a query to the scope's ownership mode.
func applyClientAccessScope(db *gorm.DB, scope ClientAccessScope) *gorm.DB {
	scope = normalizeClientAccessScope(scope)
	db = applyClientGroupAccessScope(db, scope)
	db = applyClientInboundAccessScope(db, scope)
	switch scope.Mode {
	case ClientAccessAll:
		return db
	case ClientAccessOwn:
		return db.Where("owner_admin_id = ?", scope.AdminID)
	default:
		return db.Where("1 = 0")
	}
}

// clientIdsInScope returns a subquery selecting the ids the scope allows.
//
// The paged clients query selects from "clients AS c" with joins, where a bare
// column name could bind to the wrong table. Constraining c.id against the scope
// applied to the clients table keeps that query's predicates identical to every
// other scoped query instead of duplicating them with an alias — a second copy is
// a second thing to keep correct.
func clientIdsInScope(db *gorm.DB, scope ClientAccessScope) *gorm.DB {
	// A fresh session, so the scope's predicates are rendered against a statement
	// of their own and cannot be folded into the caller's query — which is what
	// makes this safe to hand to another Where as a subquery.
	base := db.Session(&gorm.Session{NewDB: true}).Model(&model.ClientRecord{})
	return applyClientAccessScope(base, scope).Select("id")
}

// clientInboundsVisibleForScope reports whether a client is reachable through at
// least one inbound the scope allows.
//
// "At least one", not "all of them": a client attached to inbound A and inbound B
// is genuinely on A, so a role allowed to see A has to see it — that is the whole
// point of filtering the clients list by inbound, and the inbound's own client
// view lists it for the same reason. Requiring every attachment to be allowed
// would hide clients that legitimately appear on an allowed inbound.
//
// This is deliberately NOT ClientInboundsAllowedForScope, which asks the other
// question: whether an operation may target a given set of inbounds. Attaching a
// client to inbound B must still be refused when B is out of scope, even though
// the client itself is visible.
func clientInboundsVisibleForScope(scope ClientAccessScope, inboundIDs []int) bool {
	scope = normalizeClientAccessScope(scope)
	if !scope.RestrictInbounds || scope.AllowAllInbounds {
		return true
	}
	if len(scope.AllowedInboundIDs) == 0 || len(inboundIDs) == 0 {
		// Restricted to an empty set of inbounds, or the client is on none: there is
		// no allowed inbound it could be reached through.
		return false
	}
	allowed := make(map[int]struct{}, len(scope.AllowedInboundIDs))
	for _, id := range scope.AllowedInboundIDs {
		allowed[id] = struct{}{}
	}
	for _, id := range inboundIDs {
		if _, ok := allowed[id]; ok {
			return true
		}
	}
	return false
}

// ClientRecordAllowed reports whether a single client is inside the scope.
//
// inboundIDs are the inbounds the client is attached to, which the caller already
// has whenever it loaded the client's attachments. They are a parameter rather
// than a lookup so this stays a pure predicate, and so a caller cannot forget to
// supply them: the previous signature had no inbound dimension at all, which is
// how a role restricted to one inbound could still authorise any client by email.
func ClientRecordAllowed(scope ClientAccessScope, rec *model.ClientRecord, inboundIDs []int) bool {
	scope = normalizeClientAccessScope(scope)
	if rec == nil {
		return false
	}
	if !clientGroupAllowed(scope, rec.Group) {
		return false
	}
	if !clientInboundsVisibleForScope(scope, inboundIDs) {
		return false
	}
	switch scope.Mode {
	case ClientAccessAll:
		return true
	case ClientAccessOwn:
		return rec.OwnerAdminId > 0 && rec.OwnerAdminId == int64(scope.AdminID)
	default:
		return false
	}
}

// ClientInboundsAllowedForScope reports whether a scoped client operation may
// use every one of the given inbound ids.
func ClientInboundsAllowedForScope(scope ClientAccessScope, inboundIds []int) bool {
	scope = normalizeClientAccessScope(scope)
	if scope.Mode == ClientAccessNone {
		return false
	}
	if !scope.RestrictInbounds || scope.AllowAllInbounds {
		return true
	}
	if len(scope.AllowedInboundIDs) == 0 || len(inboundIds) == 0 {
		return false
	}
	allowed := make(map[int]struct{}, len(scope.AllowedInboundIDs))
	for _, id := range scope.AllowedInboundIDs {
		allowed[id] = struct{}{}
	}
	for _, id := range inboundIds {
		if id <= 0 {
			return false
		}
		if _, ok := allowed[id]; !ok {
			return false
		}
	}
	return true
}

// ClientGroupAllowedForScope reports whether a scoped operation may touch a
// client group.
func ClientGroupAllowedForScope(scope ClientAccessScope, group string) bool {
	scope = normalizeClientAccessScope(scope)
	if scope.Mode == ClientAccessNone {
		return false
	}
	return clientGroupAllowed(scope, group)
}

// ---- role document readers -------------------------------------------------

func rolePermissionValue(role *model.AdminRole, section string, key string) any {
	if role == nil {
		return nil
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(role.PermissionsJSON), &root); err != nil {
		return nil
	}
	sectionValue, ok := root[section]
	if !ok {
		return nil
	}
	sectionMap, ok := sectionValue.(map[string]any)
	if !ok {
		return nil
	}
	for _, candidate := range clientPermissionKeys(key) {
		if value, ok := sectionMap[candidate]; ok {
			return value
		}
	}
	return nil
}

func clientPermissionKeys(key string) []string {
	switch key {
	case "view":
		return []string{"view", "read"}
	case "read":
		return []string{"read", "view"}
	case "view_simple":
		return []string{"view_simple", "read_simple", "viewSimpleList", "viewSimple"}
	case "reset_usage":
		return []string{"reset_usage", "resetUsage"}
	case "revoke_sub":
		return []string{"revoke_sub", "revokeSubscription"}
	case "activate_next_plan":
		return []string{"activate_next_plan", "activateNextPlan"}
	case "admin_filter":
		return []string{"admin_filter", "adminFilter"}
	case "set_owner":
		return []string{"set_owner", "setOwner"}
	default:
		return []string{key}
	}
}

// ClientAccessModeFromPermission converts a stored permission value into an
// ownership mode: "all"/2 → all, "own"/1/true → own, false/null/0 → none.
func ClientAccessModeFromPermission(v any) ClientAccessMode {
	switch t := v.(type) {
	case map[string]any:
		return ClientAccessModeFromPermission(t["scope"])
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "all", "2":
			return ClientAccessAll
		case "own", "1":
			return ClientAccessOwn
		default:
			return ClientAccessNone
		}
	case bool:
		if t {
			return ClientAccessAll
		}
		return ClientAccessNone
	case float64:
		switch int(t) {
		case 2:
			return ClientAccessAll
		case 1:
			return ClientAccessOwn
		}
	case int:
		switch t {
		case 2:
			return ClientAccessAll
		case 1:
			return ClientAccessOwn
		}
	}
	return ClientAccessNone
}

func roleAccessValue(root map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if v, ok := root[key]; ok {
			return v, true
		}
	}
	return nil, false
}

func roleAccessBoolValue(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true", "yes", "1", "all":
			return true
		}
	case float64:
		return t != 0
	case int:
		return t != 0
	}
	return false
}

func roleAccessStringSlice(v any) []string {
	items, ok := v.([]any)
	if !ok {
		if list, ok := v.([]string); ok {
			return list
		}
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func roleAccessIntSlice(v any) []int {
	items, ok := v.([]any)
	if !ok {
		if list, ok := v.([]int); ok {
			return list
		}
		return nil
	}
	out := make([]int, 0, len(items))
	for _, item := range items {
		switch n := item.(type) {
		case float64:
			out = append(out, int(n))
		case int:
			out = append(out, n)
		}
	}
	return out
}

// roleGroupAccessScope resolves the group restriction of a role's access doc.
func roleGroupAccessScope(role *model.AdminRole) (restrict bool, allowAll bool, groups []string) {
	if role == nil {
		return true, false, nil
	}
	if role.OwnerRole {
		return true, true, nil
	}

	var root map[string]any
	if err := json.Unmarshal([]byte(role.AccessJSON), &root); err != nil {
		return true, true, nil
	}

	if value, ok := roleAccessValue(root, "allowAllGroups", "allow_all_groups"); ok && roleAccessBoolValue(value) {
		return true, true, nil
	}
	if value, ok := roleAccessValue(root, "allowedGroups", "allowed_groups"); ok {
		names := normalizeAllowedClientGroups(roleAccessStringSlice(value))
		if len(names) == 0 {
			return true, false, nil
		}
		return true, false, names
	}
	return true, true, nil
}

// roleInboundAccessScope resolves the inbound restriction of a role's access doc.
func roleInboundAccessScope(role *model.AdminRole) (restrict bool, allowAll bool, ids []int) {
	if role == nil {
		return true, false, nil
	}
	if role.OwnerRole {
		return true, true, nil
	}

	var root map[string]any
	if err := json.Unmarshal([]byte(role.AccessJSON), &root); err != nil {
		return true, true, nil
	}

	if value, ok := roleAccessValue(root, "allowAllInbounds", "allow_all_inbounds"); ok && roleAccessBoolValue(value) {
		return true, true, nil
	}
	if value, ok := roleAccessValue(root, "allowed_inbound_ids", "allowedInboundIds"); ok {
		list := normalizeAllowedInboundIDs(roleAccessIntSlice(value))
		if len(list) == 0 {
			return true, true, nil
		}
		return true, false, list
	}
	return true, true, nil
}

func adminRoleForUser(user *model.User) (*model.AdminRole, error) {
	if user == nil {
		return nil, gorm.ErrRecordNotFound
	}
	db := database.GetDB()
	if db == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var role model.AdminRole
	if err := db.Where("id = ?", user.RoleId).First(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

// ClientAccessScopeForAdmin resolves the client scope for one admin + action.
// Only the owner role may reach across admins; every non-owner role is capped
// at "own" even when its permission says "all".
func ClientAccessScopeForAdmin(user *model.User, permission string) ClientAccessScope {
	role, err := adminRoleForUser(user)
	if err != nil {
		return ClientAccessScope{Mode: ClientAccessNone, RestrictGroups: true}
	}

	restrictGroups, allowAllGroups, allowedGroups := roleGroupAccessScope(role)
	restrictInbounds, allowAllInbounds, allowedInboundIDs := roleInboundAccessScope(role)

	if role.OwnerRole {
		return normalizeClientAccessScope(ClientAccessScope{
			AdminID:           user.Id,
			Mode:              ClientAccessAll,
			RestrictGroups:    restrictGroups,
			AllowAllGroups:    allowAllGroups,
			AllowedGroups:     allowedGroups,
			RestrictInbounds:  restrictInbounds,
			AllowAllInbounds:  allowAllInbounds,
			AllowedInboundIDs: allowedInboundIDs,
		})
	}

	// A role that says "all" means all: every client on the panel, including the
	// ones other admins created. Roles that must stay inside their own records
	// ask for "own" explicitly. (Until v1.21 this was silently downgraded to
	// "own", so an admin whose role granted "all" saw an empty list whenever they
	// had not created clients themselves.)
	mode := ClientAccessModeFromPermission(rolePermissionValue(role, "users", permission))
	return normalizeClientAccessScope(ClientAccessScope{
		AdminID:           user.Id,
		Mode:              mode,
		RestrictGroups:    restrictGroups,
		AllowAllGroups:    allowAllGroups,
		AllowedGroups:     allowedGroups,
		RestrictInbounds:  restrictInbounds,
		AllowAllInbounds:  allowAllInbounds,
		AllowedInboundIDs: allowedInboundIDs,
	})
}

// ---- inbound listing scope ------------------------------------------------

// InboundAccessScope says which inbounds an account may list.
//
// The inbound list used to be filtered by the legacy `user_id` column alone, so
// an admin only ever saw inbounds they had created themselves: granting a role
// an inbound did not make it appear, and an admin who had created none saw an
// empty Inbounds page no matter what the role allowed.
//
// Resolution order, most explicit first:
//
//  1. no account (API token, unit test)      -> every inbound
//  2. the owner role                         -> every inbound
//  3. access.allowed_inbound_ids non-empty   -> exactly those ids
//  4. access document present, no id list    -> every inbound ("leave empty to
//     allow all inbounds", as the role editor states)
//  5. no usable access document at all       -> the legacy rule: only the
//     inbounds this account created, so an unknown role cannot widen access
type InboundAccessScope struct {
	// All lists every inbound on the panel.
	All bool
	// IDs is the allowed set when All is false. An empty set with All false
	// means no inbound is visible.
	IDs []int
	// LegacyUserID filters by the inbound's owner when no role document applies.
	LegacyUserID int
}

// InboundAccessScopeForAdmin resolves the inbound listing scope for one account.
func InboundAccessScopeForAdmin(user *model.User) InboundAccessScope {
	if user == nil {
		return InboundAccessScope{All: true}
	}
	role, err := adminRoleForUser(user)
	if err != nil {
		return InboundAccessScope{LegacyUserID: user.Id}
	}
	if role.OwnerRole {
		return InboundAccessScope{All: true}
	}

	root := map[string]any{}
	if err := json.Unmarshal([]byte(role.AccessJSON), &root); err != nil || len(root) == 0 {
		return InboundAccessScope{LegacyUserID: user.Id}
	}

	restrict, allowAll, ids := roleInboundAccessScope(role)
	if !restrict || allowAll {
		return InboundAccessScope{All: true}
	}
	return InboundAccessScope{IDs: ids}
}

// FilterClientEmailsForScope keeps only the emails the scope may see.
func FilterClientEmailsForScope(scope ClientAccessScope, emails []string) []string {
	if len(emails) == 0 {
		return emails
	}
	scope = normalizeClientAccessScope(scope)
	if scope.Mode == ClientAccessNone {
		return []string{}
	}

	db := database.GetDB()
	if db == nil {
		return []string{}
	}

	var rows []model.ClientRecord
	if err := applyClientAccessScope(db.Model(&model.ClientRecord{}), scope).
		Where("email IN ?", emails).
		Find(&rows).Error; err != nil {
		return []string{}
	}

	allowed := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		allowed[r.Email] = struct{}{}
	}
	out := make([]string, 0, len(allowed))
	for _, email := range emails {
		if _, ok := allowed[email]; ok {
			out = append(out, email)
		}
	}
	return out
}

// ---- controller-facing helpers ---------------------------------------------

// ClientServiceClientScope exposes the resolved scope for a service receiver so
// the HTTP layer can apply it without importing the package helpers directly.
func (s *ClientService) ClientScopeFor(user *model.User, permission string) ClientAccessScope {
	return ClientAccessScopeForAdmin(user, permission)
}

// ListForScope returns every client record the scope may see.
func (s *ClientService) ListForScope(scope ClientAccessScope) ([]ClientWithAttachments, error) {
	rows, err := s.List()
	if err != nil {
		return nil, err
	}
	scope = normalizeClientAccessScope(scope)
	if scope.Mode == ClientAccessAll && (!scope.RestrictGroups || scope.AllowAllGroups) {
		return rows, nil
	}

	emails := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Email != "" {
			emails = append(emails, row.Email)
		}
	}
	allowed := make(map[string]struct{}, len(emails))
	for _, email := range FilterClientEmailsForScope(scope, emails) {
		allowed[email] = struct{}{}
	}

	out := make([]ClientWithAttachments, 0, len(rows))
	for _, row := range rows {
		if _, ok := allowed[row.Email]; ok {
			out = append(out, row)
		}
	}
	return out, nil
}

// RequireClientForScopeByEmail loads a client and fails when the scope may not
// touch it, so mutations cannot be aimed at another admin's client.
func (s *ClientService) RequireClientForScopeByEmail(scope ClientAccessScope, email string) (*model.ClientRecord, error) {
	if strings.TrimSpace(email) == "" {
		return nil, errors.New("email is required")
	}
	db := database.GetDB()
	if db == nil {
		return nil, errors.New("database is not initialized")
	}

	var rec model.ClientRecord
	if err := applyClientAccessScope(db.Model(&model.ClientRecord{}), scope).
		Where("email = ?", email).First(&rec).Error; err != nil {
		return nil, errors.New("client not found or not permitted")
	}
	return &rec, nil
}

// RequireClientForScopeBySubID loads a client by subscription id and fails when
// the scope may not touch it.
//
// The subscription id is an opaque per-client token, which made it a way to reach
// a client without going through any lookup that checks the scope: the sub-link
// route took the id and rendered that client's links. A scoped admin had to be
// able to guess or observe a sub id to use it, but nothing else stopped them.
func (s *ClientService) RequireClientForScopeBySubID(scope ClientAccessScope, subID string) (*model.ClientRecord, error) {
	if strings.TrimSpace(subID) == "" {
		return nil, errors.New("sub id is required")
	}
	return s.requireClientForScope(scope, "sub_id = ?", subID)
}

// RequireClientForScopeByID loads a client by numeric id inside the scope. The
// Happ-link route addresses a client this way, and an id is trivially enumerable.
func (s *ClientService) RequireClientForScopeByID(scope ClientAccessScope, id int) (*model.ClientRecord, error) {
	if id <= 0 {
		return nil, errors.New("client id is required")
	}
	return s.requireClientForScope(scope, "id = ?", id)
}

func (s *ClientService) requireClientForScope(scope ClientAccessScope, cond string, arg any) (*model.ClientRecord, error) {
	db := database.GetDB()
	if db == nil {
		return nil, errors.New("database is not initialized")
	}
	var rec model.ClientRecord
	if err := applyClientAccessScope(db.Model(&model.ClientRecord{}), scope).
		Where(cond, arg).First(&rec).Error; err != nil {
		return nil, errors.New("client not found or not permitted")
	}
	return &rec, nil
}

// FilterRecordsForScope keeps the records a scope may see.
//
// For lookups that resolve a client by something other than an email — a telegram
// id can match several rows — this filters the result instead of failing the whole
// request on the first row outside the scope.
func FilterRecordsForScope(scope ClientAccessScope, records []*model.ClientRecord) []*model.ClientRecord {
	scope = normalizeClientAccessScope(scope)
	if len(records) == 0 {
		return records
	}
	if !clientScopeRestricts(scope) {
		return records
	}
	db := database.GetDB()
	if db == nil {
		return nil
	}
	ids := make([]int, 0, len(records))
	for _, rec := range records {
		if rec != nil {
			ids = append(ids, rec.Id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var allowedIDs []int
	if err := applyClientAccessScope(db.Model(&model.ClientRecord{}), scope).
		Where("id IN ?", ids).
		Pluck("id", &allowedIDs).Error; err != nil {
		return nil
	}
	allowed := make(map[int]struct{}, len(allowedIDs))
	for _, id := range allowedIDs {
		allowed[id] = struct{}{}
	}
	out := make([]*model.ClientRecord, 0, len(allowedIDs))
	for _, rec := range records {
		if rec == nil {
			continue
		}
		if _, ok := allowed[rec.Id]; ok {
			out = append(out, rec)
		}
	}
	return out
}

// UUIDsForScope returns the set of client uuids the scope may see.
//
// Several live-status endpoints are keyed by a client's guid (its uuid) rather than
// its email: which inbounds it is active on, its observed addresses. They were
// unscoped, so a role that hid a client from the clients list could still read that
// client's uuid, its addresses and the inbounds it was using from them. Keying the
// filter on the uuid lets those endpoints be narrowed by the same scope.
func (s *ClientService) UUIDsForScope(scope ClientAccessScope) (map[string]struct{}, error) {
	db := database.GetDB()
	if db == nil {
		return nil, errors.New("database is not initialized")
	}
	var uuids []string
	if err := applyClientAccessScope(db.Model(&model.ClientRecord{}), scope).
		Where("COALESCE(uuid, '') <> ''").
		Pluck("uuid", &uuids).Error; err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(uuids))
	for _, u := range uuids {
		if u != "" {
			out[u] = struct{}{}
		}
	}
	return out, nil
}

// CanCreateClientForAdmin reports whether the role may create clients.
func (s *ClientService) CanCreateClientForAdmin(user *model.User) bool {
	role, err := adminRoleForUser(user)
	if err != nil {
		return false
	}
	if role.OwnerRole {
		return true
	}
	return permissionValueAllowedInRole(role, "users", "create")
}

// permissionValueAllowedInRole reads resource.action out of a role document.
func permissionValueAllowedInRole(role *model.AdminRole, section string, action string) bool {
	value := rolePermissionValue(role, section, action)
	if value == nil {
		return false
	}
	return clientAccessModeFromAny(value) != ClientAccessNone
}

func clientAccessModeFromAny(v any) ClientAccessMode {
	switch t := v.(type) {
	case bool:
		if t {
			return ClientAccessAll
		}
		return ClientAccessNone
	case string:
		if strings.EqualFold(strings.TrimSpace(t), "true") {
			return ClientAccessAll
		}
	}
	return ClientAccessModeFromPermission(v)
}

// AssignOwnerAdmin stamps every client created by a panel account with that
// account's id, which is what makes "own"-scoped roles work. The owner role
// keeps owner 0 so those clients stay visible to every owner.
func (s *ClientService) AssignOwnerAdmin(emails []string, adminID int) error {
	if adminID <= 0 || len(emails) == 0 {
		return nil
	}
	db := database.GetDB()
	if db == nil {
		return errors.New("database is not initialized")
	}
	clean := make([]string, 0, len(emails))
	for _, email := range emails {
		if e := strings.TrimSpace(email); e != "" {
			clean = append(clean, e)
		}
	}
	if len(clean) == 0 {
		return nil
	}
	return db.Model(&model.ClientRecord{}).
		Where("email IN ? AND (owner_admin_id IS NULL OR owner_admin_id = 0)", clean).
		Update("owner_admin_id", adminID).Error
}
