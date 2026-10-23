package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
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
	g.GET("/status", requireOwnerRole(), a.status)
	g.POST("/settings", requireOwnerRole(), a.saveSettings)
	g.POST("/install", requireOwnerRole(), a.install)
	g.POST("/client/add", requireOwnerRole(), a.addClient)
	g.POST("/client/update", requireOwnerRole(), a.updateClient)
	g.POST("/client/reset", requireOwnerRole(), a.resetTraffic)
	g.POST("/client/del", requireOwnerRole(), a.delClient)
	g.POST("/client/kill", requireOwnerRole(), a.killClient)
	g.GET("/client/config", requireOwnerRole(), a.clientConfig)
	g.POST("/restart", requireOwnerRole(), a.restart)
}

func (a *OpenVPNController) status(c *gin.Context) {
	status, err := a.openvpnService.GetStatus()
	jsonObj(c, status, err)
}

type openVPNAddForm struct {
	Name       string `json:"name" form:"name"`
	Password   string `json:"password" form:"password"`
	TotalGB    int64  `json:"totalGB" form:"totalGB"`
	ExpiryTime int64  `json:"expiryTime" form:"expiryTime"`
	LimitIP    int    `json:"limitIp" form:"limitIp"`
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

	err := a.openvpnService.AddClient(name, password, form.TotalGB, form.ExpiryTime, form.LimitIP)
	jsonMsg(c, fmt.Sprintf("Client '%s' created successfully", name), err)
}

type openVPNUpdateForm struct {
	Name       string `json:"name" form:"name"`
	Password   string `json:"password" form:"password"`
	TotalGB    int64  `json:"totalGB" form:"totalGB"`
	ExpiryTime int64  `json:"expiryTime" form:"expiryTime"`
	LimitIP    int    `json:"limitIp" form:"limitIp"`
	Enable     *bool  `json:"enable" form:"enable"`
}

func (a *OpenVPNController) updateClient(c *gin.Context) {
	var form openVPNUpdateForm
	_ = c.ShouldBind(&form)

	name := strings.TrimSpace(form.Name)
	if name == "" {
		name = strings.TrimSpace(c.PostForm("name"))
	}

	password := strings.TrimSpace(form.Password)
	if password == "" {
		password = strings.TrimSpace(c.PostForm("password"))
	}

	enable := true
	if form.Enable != nil {
		enable = *form.Enable
	} else if enableStr := c.PostForm("enable"); enableStr != "" {
		enable = enableStr == "true" || enableStr == "1"
	}

	err := a.openvpnService.UpdateClient(name, password, form.TotalGB, form.ExpiryTime, form.LimitIP, enable)
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
	err := a.openvpnService.KillClient(name)
	jsonMsg(c, fmt.Sprintf("Client '%s' disconnected", name), err)
}

func (a *OpenVPNController) clientConfig(c *gin.Context) {
	name := strings.TrimSpace(c.Query("client"))
	if name == "" {
		name = strings.TrimSpace(c.Query("name"))
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
