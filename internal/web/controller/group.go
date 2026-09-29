package controller

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/gin-gonic/gin"
)

type GroupController struct {
	clientService service.ClientService
	xrayService   service.XrayService
}

func (a *GroupController) groupAllowed(c *gin.Context, group string) bool {
	_, role, ok := loginActiveAdminRole(c)
	if !ok || role == nil || role.OwnerRole {
		return true
	}
	restrict, allowAll, allowed := service.RoleGroupAccessScope(role)
	if !restrict || allowAll {
		return true
	}
	g := strings.TrimSpace(group)
	for _, it := range allowed {
		if strings.EqualFold(it, g) {
			return true
		}
	}
	return false
}

func NewGroupController(g *gin.RouterGroup) *GroupController {
	a := &GroupController{}
	a.initRouter(g)
	return a
}

func (a *GroupController) initRouter(g *gin.RouterGroup) {
	g.GET("/groups", requirePanelPermission("groups", "view"), a.list)
	g.GET("/groups/:name/emails", requirePanelPermission("groups", "view"), a.emails)
	g.POST("/groups/create", requirePanelPermission("groups", "create"), a.create)
	g.POST("/groups/rename", requirePanelPermission("groups", "update"), a.rename)
	g.POST("/groups/delete", requirePanelPermission("groups", "delete"), a.delete)
	g.POST("/groups/resetTraffic", requirePanelPermission("clients", "resetUsage"), a.resetTraffic)
	g.POST("/groups/bulkAdd", requirePanelPermission("groups", "update"), a.bulkAdd)
	g.POST("/groups/bulkRemove", requirePanelPermission("groups", "update"), a.bulkRemove)
}

func (a *GroupController) list(c *gin.Context) {
	rows, err := a.clientService.ListGroups()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	_, role, ok := loginActiveAdminRole(c)
	if ok && role != nil && !role.OwnerRole {
		restrict, allowAll, allowed := service.RoleGroupAccessScope(role)
		if restrict && !allowAll {
			allowedMap := make(map[string]bool, len(allowed))
			for _, it := range allowed {
				allowedMap[strings.ToLower(strings.TrimSpace(it))] = true
			}
			filtered := make([]model.ClientGroupRow, 0, len(rows))
			for _, r := range rows {
				if allowedMap[strings.ToLower(strings.TrimSpace(r.Name))] {
					filtered = append(filtered, r)
				}
			}
			rows = filtered
		}
	}
	jsonObj(c, rows, nil)
}

func (a *GroupController) emails(c *gin.Context) {
	name := c.Param("name")
	if !a.groupAllowed(c, name) {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), common.NewError("access denied for group"))
		return
	}
	emails, err := a.clientService.EmailsByGroup(name)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, emails, nil)
}

type groupCreateBody struct {
	Name string `json:"name"`
}

func (a *GroupController) create(c *gin.Context) {
	var body groupCreateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.clientService.CreateGroup(body.Name); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"name": body.Name}, nil)
	notifyClientsChanged()
}

type groupRenameBody struct {
	OldName string `json:"oldName"`
	NewName string `json:"newName"`
}

func (a *GroupController) rename(c *gin.Context) {
	var body groupRenameBody
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if !a.groupAllowed(c, body.OldName) || !a.groupAllowed(c, body.NewName) {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), common.NewError("access denied for group"))
		return
	}
	affected, err := a.clientService.RenameGroup(body.OldName, body.NewName)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	a.xrayService.SetToNeedRestart()
	jsonObj(c, gin.H{"affected": affected}, nil)
	notifyClientsChanged()
}

type groupDeleteBody struct {
	Name string `json:"name"`
}

func (a *GroupController) delete(c *gin.Context) {
	var body groupDeleteBody
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if !a.groupAllowed(c, body.Name) {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), common.NewError("access denied for group"))
		return
	}
	affected, err := a.clientService.DeleteGroup(body.Name)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	a.xrayService.SetToNeedRestart()
	jsonObj(c, gin.H{"affected": affected}, nil)
	notifyClientsChanged()
}

type groupResetTrafficBody struct {
	Name string `json:"name"`
}

func (a *GroupController) resetTraffic(c *gin.Context) {
	var body groupResetTrafficBody
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if !a.groupAllowed(c, body.Name) {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), common.NewError("access denied for group"))
		return
	}
	if err := a.clientService.ResetGroupTraffic(body.Name); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"name": body.Name}, nil)
	notifyClientsChanged()
}

type bulkAddToGroupRequest struct {
	Emails []string `json:"emails"`
	Group  string   `json:"group"`
}

func (a *GroupController) bulkAdd(c *gin.Context) {
	var req bulkAddToGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if strings.TrimSpace(req.Group) == "" {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), common.NewError("group name is required"))
		return
	}
	if !a.groupAllowed(c, req.Group) {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), common.NewError("access denied for group"))
		return
	}
	affected, err := a.clientService.AddToGroup(req.Emails, req.Group)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"affected": affected}, nil)
	notifyClientsChanged()
}

type bulkRemoveFromGroupRequest struct {
	Emails []string `json:"emails"`
}

func (a *GroupController) bulkRemove(c *gin.Context) {
	var req bulkRemoveFromGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	affected, err := a.clientService.RemoveFromGroup(req.Emails)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"affected": affected}, nil)
	a.xrayService.SetToNeedRestart()
	notifyClientsChanged()
}
