package controller

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	g.GET("/status", requireOwnerOrTelegramShopAccess(), a.getStatus)
	g.POST("/install", requireOwnerOrTelegramShopAccess(), a.install)
	g.POST("/install-deps", requireOwnerOrTelegramShopAccess(), a.installDeps)
	g.POST("/action", requireOwnerOrTelegramShopAccess(), a.action)
	g.POST("/restore", requireOwnerOrTelegramShopAccess(), a.restore)
	g.GET("/backup", requireOwnerOrTelegramShopAccess(), a.backup)
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

func (a *TelegramShopController) installDeps(c *gin.Context) {
	err := a.shopService.InstallDependencies()
	jsonMsg(c, "dependencies installed", err)
}

func (a *TelegramShopController) restore(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.telegramShop.restoreFileRequired"), err)
		return
	}
	defer file.Close()

	filename := header.Filename
	ext := strings.ToLower(filepath.Ext(filename))
	isTarGz := strings.HasSuffix(strings.ToLower(filename), ".tar.gz") || ext == ".tgz"
	if ext != ".sql" && !isTarGz {
		jsonMsg(c, "Invalid file format. Only .sql or .tar.gz backups are supported.", fmt.Errorf("invalid extension"))
		return
	}

	targetExt := ext
	if isTarGz {
		targetExt = ".tar.gz"
	}

	tmpFile, err := os.CreateTemp("", "tg_restore_*"+targetExt)
	if err != nil {
		jsonMsg(c, "Failed to create temporary file", err)
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, file); err != nil {
		_ = tmpFile.Close()
		jsonMsg(c, "Failed to write backup file", err)
		return
	}
	_ = tmpFile.Close()

	if err := a.shopService.RestoreBackup(c.Request.Context(), tmpPath); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.telegramShop.restoreFailed"), err)
		return
	}

	jsonMsg(c, I18nWeb(c, "pages.telegramShop.restoreSuccess"), nil)
}

func (a *TelegramShopController) backup(c *gin.Context) {
	filePath, err := a.shopService.CreateBackup(c.Request.Context())
	if err != nil {
		jsonMsg(c, "Failed to create bot backup", err)
		return
	}
	defer os.Remove(filePath)

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	c.FileAttachment(filePath, fmt.Sprintf("novabot_backup_%s.sql", timestamp))
}

