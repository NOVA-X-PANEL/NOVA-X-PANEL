package controller

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// HashemController manages Hashem GRE + FRP reverse tunnels.
type HashemController struct {
	BaseController
	hashemService service.HashemService
}

// NewHashemController instantiates HashemController and mounts its routes.
func NewHashemController(g *gin.RouterGroup) *HashemController {
	a := &HashemController{}
	a.initRouter(g)
	return a
}

func (a *HashemController) initRouter(g *gin.RouterGroup) {
	g.GET("/status", a.status)
	g.POST("/setup", a.setup)
	g.POST("/setup-ssh", a.setupSSH)
	g.POST("/generate-oneliner", a.generateOneLiner)
	g.POST("/carrier", a.carrier)
	g.POST("/restart", a.restart)
	g.POST("/watchdog", a.watchdog)
	g.POST("/sync-inbounds", a.syncInbounds)
	g.POST("/install", a.install)
	g.POST("/remove", a.remove)
}

func (a *HashemController) status(c *gin.Context) {
	status, err := a.hashemService.GetStatus()
	jsonObj(c, status, err)
}

func (a *HashemController) setup(c *gin.Context) {
	var form service.HashemSetupForm
	if err := c.ShouldBindJSON(&form); err != nil {
		jsonMsg(c, "invalid setup payload", err)
		return
	}
	out, err := a.hashemService.Setup(form)
	jsonObj(c, out, err)
}

func (a *HashemController) carrier(c *gin.Context) {
	var payload struct {
		Carrier string `json:"carrier"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil || payload.Carrier == "" {
		jsonMsg(c, "carrier mode required", errors.New("missing carrier"))
		return
	}
	err := a.hashemService.SetCarrier(payload.Carrier)
	jsonMsg(c, "carrier updated", err)
}

func (a *HashemController) restart(c *gin.Context) {
	err := a.hashemService.Restart()
	jsonMsg(c, "hashem tunnel restarted", err)
}

func (a *HashemController) watchdog(c *gin.Context) {
	var payload struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		jsonMsg(c, "invalid watchdog payload", err)
		return
	}
	err := a.hashemService.SetWatchdog(payload.Enabled)
	jsonMsg(c, "watchdog updated", err)
}

func (a *HashemController) syncInbounds(c *gin.Context) {
	ports, err := a.hashemService.SyncInbounds()
	jsonObj(c, ports, err)
}

func (a *HashemController) install(c *gin.Context) {
	out, err := a.hashemService.Install()
	jsonObj(c, out, err)
}

func (a *HashemController) remove(c *gin.Context) {
	err := a.hashemService.Remove()
	jsonMsg(c, "tunnel removed", err)
}

func (a *HashemController) setupSSH(c *gin.Context) {
	var form service.HashemSSHSetupForm
	if err := c.ShouldBind(&form); err != nil {
		jsonMsg(c, "داده‌های اتصال SSH نامعتبر است", err)
		return
	}
	res, err := a.hashemService.SetupSSH(form)
	jsonObj(c, res, err)
}

func (a *HashemController) generateOneLiner(c *gin.Context) {
	var form service.HashemOneLinerForm
	if err := c.ShouldBind(&form); err != nil {
		jsonMsg(c, "داده‌های تولید دستور نامعتبر است", err)
		return
	}
	res, err := a.hashemService.GenerateOneLiner(form)
	jsonObj(c, res, err)
}
