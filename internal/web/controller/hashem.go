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
	g.GET("/status", requireOwnerRole(), a.status)
	g.POST("/setup", requireOwnerRole(), a.setup)
	g.POST("/setup-ssh", requireOwnerRole(), a.setupSSH)
	g.POST("/generate-oneliner", requireOwnerRole(), a.generateOneLiner)
	g.POST("/carrier", requireOwnerRole(), a.carrier)
	g.POST("/restart", requireOwnerRole(), a.restart)
	g.POST("/watchdog", requireOwnerRole(), a.watchdog)
	g.POST("/sync-inbounds", requireOwnerRole(), a.syncInbounds)
	g.POST("/edit-ports", requireOwnerRole(), a.editPorts)
	g.POST("/install", requireOwnerRole(), a.install)
	g.POST("/remove", requireOwnerRole(), a.remove)
	g.GET("/benchmark", requireOwnerRole(), a.benchmark)
	g.POST("/benchmark/run", requireOwnerRole(), a.runBenchmark)
	g.POST("/benchmark/autopilot", requireOwnerRole(), a.autoPilot)
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

func (a *HashemController) editPorts(c *gin.Context) {
	var form struct {
		Ports []int `json:"ports"`
	}
	if err := c.ShouldBindJSON(&form); err != nil {
		jsonMsg(c, "invalid edit-ports payload", err)
		return
	}
	if len(form.Ports) == 0 {
		jsonMsg(c, "ports list cannot be empty", errors.New("empty ports"))
		return
	}
	err := a.hashemService.EditPorts(form.Ports)
	jsonMsg(c, "hashem ports updated", err)
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

func (a *HashemController) benchmark(c *gin.Context) {
	rep, err := a.hashemService.GetBenchmark()
	if err != nil {
		jsonMsg(c, "failed to get benchmark", err)
		return
	}
	if rep == nil {
		rep, err = a.hashemService.RunBenchmark()
	}
	jsonObj(c, rep, err)
}

func (a *HashemController) runBenchmark(c *gin.Context) {
	rep, err := a.hashemService.RunBenchmark()
	jsonObj(c, rep, err)
}

func (a *HashemController) autoPilot(c *gin.Context) {
	var payload struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		jsonMsg(c, "invalid autopilot payload", err)
		return
	}
	err := a.hashemService.SetAutoPilot(payload.Enabled)
	jsonMsg(c, "autopilot setting updated", err)
}
