package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// TelegramShopController manages the Telegram Shop bot installation, status, and lifecycle.
type TelegramShopController struct {
	BaseController
	shopService service.TelegramShopService
}

// NewTelegramShopController instantiates TelegramShopController and mounts its routes.
func NewTelegramShopController(g *gin.RouterGroup) *TelegramShopController {
	a := &TelegramShopController{}
	a.initRouter(g)
	return a
}

func (a *TelegramShopController) initRouter(g *gin.RouterGroup) {
	g.GET("/status", requireOwnerRole(), a.getStatus)
	g.POST("/install", requireOwnerRole(), a.install)
	g.POST("/action", requireOwnerRole(), a.action)
}

func (a *TelegramShopController) getStatus(c *gin.Context) {
	status, err := a.shopService.GetStatus()
	jsonObj(c, status, err)
}

func (a *TelegramShopController) install(c *gin.Context) {
	var cfg service.TelegramShopConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		jsonMsg(c, "invalid install payload", err)
		return
	}
	err := a.shopService.Install(&cfg)
	jsonMsg(c, "telegram shop installed", err)
}

func (a *TelegramShopController) action(c *gin.Context) {
	var req struct {
		Action string `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, "invalid action payload", err)
		return
	}
	err := a.shopService.Action(req.Action)
	jsonMsg(c, "action executed", err)
}
