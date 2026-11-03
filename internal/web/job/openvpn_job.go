package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// OpenVPNJob periodically syncs active OpenVPN session traffic deltas into the database
// and enforces CCD client block rules (quota exceeded, expired, or admin disabled).
type OpenVPNJob struct {
	openvpnService service.OpenVPNService
}

func NewOpenVPNJob() *OpenVPNJob {
	return &OpenVPNJob{}
}

func (j *OpenVPNJob) Run() {
	j.openvpnService.SyncTrafficAndEnforceLimits()
}
