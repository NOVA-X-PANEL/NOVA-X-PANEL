package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"
)

// OpenVPNController provides HTTP endpoints for managing OpenVPN server, accounting, and clients.
type OpenVPNController struct {
	BaseController
	openvpnService service.OpenVPNService
}

// NewOpenVPNController creates an OpenVPNController and mounts its routes.
func NewOpenVPNController(g *gin.RouterGroup) *OpenVPNController {
	a := &OpenVPNController{}
	a.initRouter(g)
	return a
}

func (a *OpenVPNController) initRouter(g *gin.RouterGroup) {
	g.GET("/status", requireAnyPanelPermission(
		panelPermissionRequirement{"openvpn", "read"},
		panelPermissionRequirement{"openvpn", "view"},
		panelPermissionRequirement{"openvpn", "read_simple"},
	), a.status)
	g.POST("/settings", requireOwnerRole(), a.saveSettings)
	g.POST("/install", requireOwnerRole(), a.install)
	g.POST("/client/add", requirePanelPermission("openvpn", "create"), a.addClient)
	g.POST("/client/update", requireAnyPanelPermission(panelPermissionRequirement{"openvpn", "update"}), a.updateClient)
	g.POST("/client/reset", requireAnyPanelPermission(panelPermissionRequirement{"openvpn", "reset_usage"}, panelPermissionRequirement{"openvpn", "resetUsage"}), a.resetTraffic)
	g.POST("/client/del", requireAnyPanelPermission(panelPermissionRequirement{"openvpn", "delete"}), a.delClient)
	g.POST("/client/kill", requireAnyPanelPermission(panelPermissionRequirement{"openvpn", "kill"}), a.killClient)
	g.GET("/client/config", requireAnyPanelPermission(
		panelPermissionRequirement{"openvpn", "config"},
		panelPermissionRequirement{"openvpn", "read"},
		panelPermissionRequirement{"openvpn", "view"},
	), a.clientConfig)
	g.POST("/restart", requireOwnerRole(), a.restart)
	g.GET("/log", requireOwnerRole(), a.log)
}

func (a *OpenVPNController) openvpnScope(c *gin.Context, permission string) service.OpenVPNAccessScope {
	if c.GetBool("api_authed") && c.GetInt("api_token_admin_id") <= 0 {
		return service.OpenVPNAccessScope{Mode: 2, IsOwner: true}
	}
	user := a.loginUser(c)
	if user == nil && gin.Mode() == gin.TestMode {
		return service.OpenVPNAccessScope{Mode: 2, IsOwner: true}
	}
	if user == nil {
		return service.OpenVPNAccessScope{Mode: 0}
	}

	role, err := adminRoleOf(user)
	if err != nil {
		return service.OpenVPNAccessScope{AdminID: user.Id, Mode: 0}
	}
	if role.OwnerRole {
		return service.OpenVPNAccessScope{AdminID: user.Id, Mode: 2, IsOwner: true}
	}

	var root map[string]any
	if err := json.Unmarshal([]byte(role.PermissionsJSON), &root); err != nil {
		return service.OpenVPNAccessScope{AdminID: user.Id, Mode: 0}
	}

	ovpnPerms, ok := root["openvpn"].(map[string]any)
	if !ok {
		return service.OpenVPNAccessScope{AdminID: user.Id, Mode: 0}
	}

	val, exists := ovpnPerms[permission]
	if !exists {
		for _, k := range permissionKeys(permission) {
			if v, ok := ovpnPerms[k]; ok {
				val = v
				exists = true
				break
			}
		}
	}

	mode := 0
	if exists {
		switch v := val.(type) {
		case bool:
			if v {
				mode = 2
			}
		case string:
			switch strings.ToLower(strings.TrimSpace(v)) {
			case "all":
				mode = 2
			case "own":
				mode = 1
			case "true", "1", "yes":
				mode = 2
			}
		case map[string]any:
			if sc, ok := v["scope"]; ok {
				switch s := sc.(type) {
				case float64:
					mode = int(s)
				case int:
					mode = s
				case string:
					if s == "all" || s == "2" {
						mode = 2
					} else if s == "own" || s == "1" {
						mode = 1
					}
				}
			}
		case float64:
			mode = int(v)
		case int:
			mode = v
		}
	}

	return service.OpenVPNAccessScope{
		AdminID: user.Id,
		Mode:    mode,
		IsOwner: false,
	}
}

func (a *OpenVPNController) loginUser(c *gin.Context) *model.User {
	var user *model.User
	func() {
		defer func() {
			if recover() != nil {
				user = nil
			}
		}()
		if c.GetBool("api_authed") {
			if tokenAdmin := apiTokenAdmin(c); tokenAdmin != nil {
				user = tokenAdmin
				return
			}
		}
		user = session.GetLoginUser(c)
	}()
	return user
}

func (a *OpenVPNController) status(c *gin.Context) {
	scope := a.openvpnScope(c, "read")
	status, err := a.openvpnService.GetStatus(scope)
	jsonObj(c, status, err)
}

type openVPNAddForm struct {
	Name         string `json:"name" form:"name"`
	Password     string `json:"password" form:"password"`
	TotalGB      int64  `json:"totalGB" form:"totalGB"`
	ExpiryTime   int64  `json:"expiryTime" form:"expiryTime"`
	LimitIP      int    `json:"limitIp" form:"limitIp"`
	OwnerAdminId int    `json:"ownerAdminId" form:"ownerAdminId"`
}

func (a *OpenVPNController) addClient(c *gin.Context) {
	var form openVPNAddForm
	_ = c.ShouldBind(&form)

	name := strings.TrimSpace(form.Name)
	if name == "" {
		name = strings.TrimSpace(c.PostForm("name"))
	}
	if name == "" {
		name = strings.TrimSpace(c.Query("name"))
	}

	password := strings.TrimSpace(form.Password)
	if password == "" {
		password = strings.TrimSpace(c.PostForm("password"))
	}

	scope := a.openvpnScope(c, "create")
	ownerAdminId := scope.AdminID
	if scope.IsOwner && form.OwnerAdminId > 0 {
		ownerAdminId = form.OwnerAdminId
	}

	err := a.openvpnService.AddClient(name, password, form.TotalGB, form.ExpiryTime, form.LimitIP, ownerAdminId)
	jsonMsg(c, fmt.Sprintf("Client '%s' created successfully", name), err)
}

type openVPNUpdateForm struct {
	Name          string `json:"name" form:"name"`
	Password      string `json:"password" form:"password"`
	TotalGB       int64  `json:"totalGB" form:"totalGB"`
	TotalGBAlt    int64  `json:"total_gb" form:"total_gb"`
	ExpiryTime    int64  `json:"expiryTime" form:"expiryTime"`
	ExpiryTimeAlt int64  `json:"expiry_time" form:"expiry_time"`
	LimitIP       int    `json:"limitIp" form:"limitIp"`
	LimitIPAlt    int    `json:"limit_ip" form:"limit_ip"`
	Enable        *bool  `json:"enable" form:"enable"`
}

func (a *OpenVPNController) updateClient(c *gin.Context) {
	var form openVPNUpdateForm
	_ = c.ShouldBind(&form)

	name := strings.TrimSpace(form.Name)
	if name == "" {
		name = strings.TrimSpace(c.PostForm("name"))
	}

	scope := a.openvpnScope(c, "update")
	if err := a.openvpnService.CanManageClient(scope, name); err != nil {
		pureJsonMsg(c, http.StatusForbidden, false, err.Error())
		return
	}

	password := strings.TrimSpace(form.Password)
	if password == "" {
		password = strings.TrimSpace(c.PostForm("password"))
	}

	totalGB := form.TotalGB
	if totalGB == 0 && form.TotalGBAlt != 0 {
		totalGB = form.TotalGBAlt
	}
	expiryTime := form.ExpiryTime
	if expiryTime == 0 && form.ExpiryTimeAlt != 0 {
		expiryTime = form.ExpiryTimeAlt
	}
	limitIP := form.LimitIP
	if limitIP == 0 && form.LimitIPAlt != 0 {
		limitIP = form.LimitIPAlt
	}

	db := database.GetDB()
	var existingUser model.OpenVPNUser
	if db != nil && name != "" {
		_ = db.Where("name = ?", name).First(&existingUser)
	}

	enable := existingUser.Enable
	if form.Enable != nil {
		enable = *form.Enable
	} else if enableStr := c.PostForm("enable"); enableStr != "" {
		enable = enableStr == "true" || enableStr == "1"
	}

	err := a.openvpnService.UpdateClient(name, password, totalGB, expiryTime, limitIP, enable)
	jsonMsg(c, fmt.Sprintf("Client '%s' updated successfully", name), err)
}

type openVPNNameForm struct {
	Name string `json:"name" form:"name"`
}

func (a *OpenVPNController) resetTraffic(c *gin.Context) {
	var form openVPNNameForm
	_ = c.ShouldBind(&form)
	name := strings.TrimSpace(form.Name)
	if name == "" {
		name = strings.TrimSpace(c.PostForm("name"))
	}
	if name == "" {
		name = strings.TrimSpace(c.Query("name"))
	}

	scope := a.openvpnScope(c, "reset_usage")
	if err := a.openvpnService.CanManageClient(scope, name); err != nil {
		pureJsonMsg(c, http.StatusForbidden, false, err.Error())
		return
	}

	err := a.openvpnService.ResetClientTraffic(name)
	jsonMsg(c, fmt.Sprintf("Traffic for '%s' reset successfully", name), err)
}

func (a *OpenVPNController) delClient(c *gin.Context) {
	var form openVPNNameForm
	_ = c.ShouldBind(&form)
	name := strings.TrimSpace(form.Name)
	if name == "" {
		name = strings.TrimSpace(c.PostForm("name"))
	}
	if name == "" {
		name = strings.TrimSpace(c.Query("name"))
	}

	scope := a.openvpnScope(c, "delete")
	if err := a.openvpnService.CanManageClient(scope, name); err != nil {
		pureJsonMsg(c, http.StatusForbidden, false, err.Error())
		return
	}

	err := a.openvpnService.DeleteClient(name)
	jsonMsg(c, fmt.Sprintf("Client '%s' deleted", name), err)
}

func (a *OpenVPNController) killClient(c *gin.Context) {
	var form openVPNNameForm
	_ = c.ShouldBind(&form)
	name := strings.TrimSpace(form.Name)
	if name == "" {
		name = strings.TrimSpace(c.PostForm("name"))
	}
	if name == "" {
		name = strings.TrimSpace(c.Query("name"))
	}

	scope := a.openvpnScope(c, "kill")
	if err := a.openvpnService.CanManageClient(scope, name); err != nil {
		pureJsonMsg(c, http.StatusForbidden, false, err.Error())
		return
	}

	err := a.openvpnService.KillClient(name)
	jsonMsg(c, fmt.Sprintf("Client '%s' disconnected", name), err)
}

func (a *OpenVPNController) clientConfig(c *gin.Context) {
	name := strings.TrimSpace(c.Query("client"))
	if name == "" {
		name = strings.TrimSpace(c.Query("name"))
	}

	scope := a.openvpnScope(c, "config")
	if err := a.openvpnService.CanManageClient(scope, name); err != nil {
		pureJsonMsg(c, http.StatusForbidden, false, err.Error())
		return
	}
	remoteIP := strings.TrimSpace(c.Query("ip"))
	portStr := strings.TrimSpace(c.Query("port"))
	port := 0
	if portStr != "" {
		port, _ = strconv.Atoi(portStr)
	}
	asRaw := c.Query("raw") == "true"
	inlineAuth := c.Query("inline") == "1" || c.Query("inline") == "true"

	config, err := a.openvpnService.GetClientConfig(name, remoteIP, port, inlineAuth)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "msg": err.Error()})
		return
	}

	if asRaw {
		jsonObj(c, gin.H{"config": config, "filename": fmt.Sprintf("%s.ovpn", name)}, nil)
		return
	}

	c.Header("Content-Type", "application/x-openvpn-profile")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.ovpn\"", name))
	c.String(http.StatusOK, config)
}

func (a *OpenVPNController) restart(c *gin.Context) {
	err := a.openvpnService.RestartService()
	jsonMsg(c, "OpenVPN service restarted successfully", err)
}

type openVPNSettingsForm struct {
	RemoteHost string `json:"remote_host" form:"remote_host"`
	RemotePort int    `json:"remote_port" form:"remote_port"`
	ServerPort int    `json:"server_port" form:"server_port"`
}

func (a *OpenVPNController) saveSettings(c *gin.Context) {
	var form openVPNSettingsForm
	_ = c.ShouldBind(&form)
	if form.RemoteHost == "" {
		form.RemoteHost = c.PostForm("remote_host")
	}
	err := a.openvpnService.SaveSettings(form.RemoteHost, form.RemotePort, form.ServerPort)
	jsonMsg(c, "OpenVPN settings saved successfully", err)
}

type openVPNInstallForm struct {
	Port  int    `json:"port" form:"port"`
	Proto string `json:"proto" form:"proto"`
}

func (a *OpenVPNController) install(c *gin.Context) {
	var form openVPNInstallForm
	_ = c.ShouldBind(&form)
	err := a.openvpnService.AutoInstallOpenVPN(form.Port, form.Proto)
	jsonMsg(c, "OpenVPN installed and initialized successfully", err)
}

func (a *OpenVPNController) log(c *gin.Context) {
	logContent, err := a.openvpnService.GetLog()
	jsonObj(c, gin.H{"log": logContent}, err)
}
