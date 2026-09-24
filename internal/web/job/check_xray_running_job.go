// Package job provides background job implementations for the 3x-ui web panel,
// including traffic monitoring, system checks, and periodic maintenance tasks.
package job

import (
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// EventBus is set from web layer to publish events.
var EventBus *eventbus.Bus

// Restart backoff.
//
// This job runs once a second, and it used to retry a failed restart on every
// second pass — so a core that cannot start was re-serialised and re-executed
// every two seconds, forever, with no ceiling.
//
// Measured on a panel with zero clients and an Xray that could not start: 15
// restart attempts and 15 rewrites of bin/config.json per 30 seconds, 1.3% of a
// core held continuously (roughly 1,100 CPU-seconds and 130,000 log lines per
// day) — for a panel doing nothing. On a real panel every attempt also
// re-serialises the whole config, so the cost scales with the client count: at
// a few hundred clients that is a large JSON document written to disk every two
// seconds, and each write is another fsync at synchronous=FULL.
//
// The delay now doubles per consecutive failure up to a cap. A crash that one
// restart fixes still recovers in about the same time as before, while a core
// that cannot start at all settles into roughly 288 attempts a day instead of
// 43,200 — a 99% reduction in the failing state, which is exactly when the panel
// is least useful and most likely to be left running unattended.
const (
	xrayRestartBackoffBase = 2 * time.Second
	xrayRestartBackoffMax  = 5 * time.Minute
)

// CheckXrayRunningJob monitors Xray process health and restarts it if it crashes.
type CheckXrayRunningJob struct {
	xrayService service.XrayService
	checkTime   int

	// consecutiveFailures counts restart attempts that did not leave the core
	// running. It drives the backoff and resets as soon as the core is up.
	consecutiveFailures int
	// attempted is true between issuing a restart and the next health check, so a
	// restart that reports success but still leaves the core down is counted too.
	// A core that starts and then exits on its own config error takes that path:
	// RestartXray returns nil, and only the next check reveals it.
	attempted bool
	// nextAttempt is the earliest time the next restart may be issued; the zero
	// value means immediately.
	nextAttempt time.Time
}

// NewCheckXrayRunningJob creates a new Xray health check job instance.
func NewCheckXrayRunningJob() *CheckXrayRunningJob {
	return new(CheckXrayRunningJob)
}

// Run checks if Xray has crashed and restarts it after confirming it's down for 2 consecutive checks.
func (j *CheckXrayRunningJob) Run() {
	if !j.xrayService.DidXrayCrash() {
		j.checkTime = 0
		if j.consecutiveFailures > 0 {
			logger.Infof("xray is running again after %d failed restart attempt(s)", j.consecutiveFailures)
		}
		j.consecutiveFailures = 0
		j.attempted = false
		j.nextAttempt = time.Time{}
		return
	}

	j.checkTime++
	// only restart if it's down 2 times in a row
	if j.checkTime <= 1 {
		return
	}
	j.checkTime = 0

	// The previous attempt did not bring the core up. Count it and start backing
	// off. RestartXray itself could not report this case: the process launched,
	// so the call succeeded, and only a later health check shows it is gone.
	if j.attempted {
		j.consecutiveFailures++
		j.attempted = false
		j.scheduleNextAttempt()
	}

	// Respect the current backoff before spending another attempt, each of which
	// rewrites the config and forks a process.
	if !j.nextAttempt.IsZero() && time.Now().Before(j.nextAttempt) {
		return
	}

	err := j.xrayService.RestartXray(false)
	j.attempted = true
	if err != nil {
		j.consecutiveFailures++
		j.attempted = false
		delay := j.scheduleNextAttempt()
		logger.Warningf(
			"restart xray failed (%d consecutive failure(s)); next attempt in %s: %v",
			j.consecutiveFailures, delay, err,
		)
	}
}

// scheduleNextAttempt records when the next restart may run, doubling the delay
// per consecutive failure up to xrayRestartBackoffMax, and returns that delay.
// The loop doubles rather than shifting so a large failure count cannot overflow.
func (j *CheckXrayRunningJob) scheduleNextAttempt() time.Duration {
	delay := xrayRestartBackoffBase
	for i := 1; i < j.consecutiveFailures && delay < xrayRestartBackoffMax; i++ {
		delay *= 2
	}
	if delay > xrayRestartBackoffMax {
		delay = xrayRestartBackoffMax
	}
	j.nextAttempt = time.Now().Add(delay)
	return delay
}
