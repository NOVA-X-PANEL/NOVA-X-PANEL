package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"gorm.io/gorm"
)

// TransferClientToAdmin changes the panel owner of an existing client without
// changing its credentials, traffic, expiry, protocol settings, or runtime
// configuration. The owner is mirrored into each attached inbound's stored
// client entry so a later inbound sync cannot restore the previous assignment.
// The linked OpenVPN account (if present) follows the same owner. Remote nodes
// are marked dirty so their panel metadata converges through the normal sync.
func (s *ClientService) TransferClientToAdmin(email string, target *model.User) (int64, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return 0, errors.New("client email is required")
	}
	if target == nil {
		return 0, errors.New("target admin is required")
	}
	targetAdminID := target.Id
	if targetAdminID <= 0 {
		return 0, errors.New("target admin id must be positive")
	}
	if database.GetDB() == nil {
		return 0, errors.New("database is not initialized")
	}
	viewScopes := []ClientAccessScope{
		ClientAccessScopeForAdmin(target, "view"),
		ClientAccessScopeForAdmin(target, "view_simple"),
	}

	var previousOwner int64
	err := runSerializedTx(func(tx *gorm.DB) error {
		var record model.ClientRecord
		if err := tx.Where("email = ?", email).First(&record).Error; err != nil {
			return err
		}
		previousOwner = record.OwnerAdminId
		if previousOwner == int64(targetAdminID) {
			return nil
		}
		var freshTarget model.User
		if err := tx.Where("id = ?", targetAdminID).First(&freshTarget).Error; err != nil {
			return errors.New("target admin not found")
		}
		if freshTarget.Status != "" && freshTarget.Status != model.AdminStatusActive {
			return errors.New("target admin is disabled")
		}
		var targetRole model.AdminRole
		if err := tx.Where("id = ?", freshTarget.RoleId).First(&targetRole).Error; err != nil {
			return errors.New("target admin role not found")
		}
		if err := validateClientTransferLimits(tx, &freshTarget, &targetRole); err != nil {
			return err
		}

		var inboundIDs []int
		if err := tx.Model(&model.ClientInbound{}).
			Where("client_id = ?", record.Id).
			Order("inbound_id ASC").
			Pluck("inbound_id", &inboundIDs).Error; err != nil {
			return err
		}
		candidate := record
		candidate.OwnerAdminId = int64(targetAdminID)
		visible := false
		for _, scope := range viewScopes {
			if ClientRecordAllowed(scope, &candidate, inboundIDs) {
				visible = true
				break
			}
		}
		if !visible {
			return errors.New("target admin cannot view this client with its current role and access rules")
		}

		if len(inboundIDs) > 0 {
			var inbounds []model.Inbound
			if err := tx.Where("id IN ?", inboundIDs).Find(&inbounds).Error; err != nil {
				return err
			}
			if len(inbounds) != len(inboundIDs) {
				return errors.New("client has a missing inbound attachment")
			}
			for i := range inbounds {
				updatedSettings, found, err := transferOwnerInSettings(inbounds[i].Settings, record.Email, targetAdminID)
				if err != nil {
					return fmt.Errorf("inbound %d settings: %w", inbounds[i].Id, err)
				}
				if !found {
					return fmt.Errorf("client %q is missing from attached inbound %d", record.Email, inbounds[i].Id)
				}
				if err := tx.Model(&model.Inbound{}).
					Where("id = ?", inbounds[i].Id).
					UpdateColumn("settings", updatedSettings).Error; err != nil {
					return err
				}
				if inbounds[i].NodeID != nil {
					if err := (&NodeService{}).MarkNodeDirtyTx(tx, *inbounds[i].NodeID); err != nil {
						return err
					}
				}
			}
		}

		result := tx.Model(&model.ClientRecord{}).
			Where("id = ? AND owner_admin_id = ?", record.Id, previousOwner).
			UpdateColumn("owner_admin_id", targetAdminID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("client ownership changed concurrently; reload and retry")
		}

		// OpenVPN accounts created for a subscription-backed client are separate
		// rows but share its email. Reassign all matching rows in the same
		// transaction; an email with no linked OpenVPN account is a normal case.
		if err := tx.Model(&model.OpenVPNUser{}).
			Where("email = ?", record.Email).
			UpdateColumn("owner_admin_id", targetAdminID).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return previousOwner, err
	}
	return previousOwner, nil
}

// ValidateClientTransferForAdmin preserves the recipient's existing client and
// traffic limits without requiring users.create: a transfer is not a new
// protocol client, but it does add one owned record to the recipient.
func (s *ClientService) ValidateClientTransferForAdmin(user *model.User) error {
	if user == nil {
		return errors.New("target admin is required")
	}
	role, err := adminRoleForUser(user)
	if err != nil {
		return err
	}
	return validateClientTransferLimits(database.GetDB(), user, role)
}

func validateClientTransferLimits(db *gorm.DB, user *model.User, role *model.AdminRole) error {
	if db == nil || user == nil {
		return errors.New("target admin or database is unavailable")
	}
	if role == nil || role.OwnerRole {
		return errors.New("target must be a non-owner admin")
	}
	if user.DataLimit > 0 && user.UsedBytes >= user.DataLimit {
		return fmt.Errorf("admin traffic quota exhausted: used %s of %s", formatBytes(user.UsedBytes), formatBytes(user.DataLimit))
	}

	var maxUsers int64
	if strings.TrimSpace(user.PermissionOverridesJSON) != "" {
		var overrides map[string]any
		if json.Unmarshal([]byte(user.PermissionOverridesJSON), &overrides) == nil && overrides != nil {
			maxUsers, _ = parseLimitInt64(overrides["max_users"])
		}
	}
	if maxUsers == 0 && strings.TrimSpace(role.LimitsJSON) != "" {
		var limits map[string]any
		if json.Unmarshal([]byte(role.LimitsJSON), &limits) == nil && limits != nil {
			maxUsers, _ = parseLimitInt64(limits["max_users"])
		}
	}
	if maxUsers <= 0 {
		return nil
	}
	var count int64
	if err := db.Model(&model.ClientRecord{}).Where("owner_admin_id = ?", user.Id).Count(&count).Error; err != nil {
		return err
	}
	if count+1 > maxUsers {
		return fmt.Errorf("client limit reached: this admin role allows at most %d clients (currently has %d)", maxUsers, count)
	}
	return nil
}

// transferOwnerInSettings changes only ownerAdminId on client entries matching
// the stable client email. Raw-message maps preserve all unrelated settings.
func transferOwnerInSettings(settings, email string, targetAdminID int) (string, bool, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal([]byte(settings), &root); err != nil {
		return "", false, err
	}
	rawClients, ok := root["clients"]
	if !ok {
		return "", false, errors.New("clients array is missing")
	}
	var clients []json.RawMessage
	if err := json.Unmarshal(rawClients, &clients); err != nil {
		return "", false, err
	}

	ownerJSON, err := json.Marshal(targetAdminID)
	if err != nil {
		return "", false, err
	}
	found := false
	for i, raw := range clients {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return "", false, err
		}
		var clientEmail string
		if err := json.Unmarshal(fields["email"], &clientEmail); err != nil {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(clientEmail), email) {
			continue
		}
		fields["ownerAdminId"] = ownerJSON
		clients[i], err = json.Marshal(fields)
		if err != nil {
			return "", false, err
		}
		found = true
	}
	if !found {
		return settings, false, nil
	}
	root["clients"], err = json.Marshal(clients)
	if err != nil {
		return "", false, err
	}
	updated, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", false, err
	}
	return string(updated), true, nil
}
