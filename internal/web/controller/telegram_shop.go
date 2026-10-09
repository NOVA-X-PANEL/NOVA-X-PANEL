package controller

import (
	"fmt"
	"strings"

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
	contentType := c.ContentType()

	if strings.Contains(contentType, "application/json") {
		if err := c.ShouldBindJSON(&cfg); err != nil {
			jsonMsg(c, "invalid install payload", err)
			return
		}
	} else {
		_ = c.ShouldBind(&cfg)
		if cfg.BotToken == "" {
			cfg.BotToken = c.PostForm("bot_token")
		}
		if cfg.AdminChatID == "" {
			cfg.AdminChatID = c.PostForm("admin_chat_id")
		}
		if cfg.CardNumber == "" {
			cfg.CardNumber = c.PostForm("card_number")
		}
		if cfg.CardHolder == "" {
			cfg.CardHolder = c.PostForm("card_holder")
		}
	}

	cfg.BotToken = strings.TrimSpace(cfg.BotToken)
	cfg.AdminChatID = strings.TrimSpace(cfg.AdminChatID)

	if cfg.BotToken == "" || cfg.AdminChatID == "" {
		jsonMsg(c, "bot_token and admin_chat_id are required", fmt.Errorf("missing credentials"))
		return
	}

	err := a.shopService.Install(&cfg)
	jsonMsg(c, "telegram shop installed", err)
}

func (a *TelegramShopController) action(c *gin.Context) {
	action := ""
	contentType := c.ContentType()

	if strings.Contains(contentType, "application/json") {
		var req struct {
			Action string `json:"action"`
		}
		if err := c.ShouldBindJSON(&req); err == nil {
			action = req.Action
		}
	}
	if action == "" {
		action = c.PostForm("action")
	}

	action = strings.TrimSpace(action)
	if action == "" {
		jsonMsg(c, "action is required", fmt.Errorf("empty action"))
		return
	}

	err := a.shopService.Action(action)
	jsonMsg(c, "action executed", err)
}
