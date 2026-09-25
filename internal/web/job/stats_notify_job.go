package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/tgbot"
)

// reportSender is the part of the Telegram service this job needs.
//
// It exists so the job's one decision — report, always — can be tested. The job
// held a concrete tgbot.Tgbot before, which left the guard below untestable, and
// an untestable guard is how it went unnoticed in the first place.
type reportSender interface {
	SendReport()
}

// StatsNotifyJob sends the periodic statistics report to the Telegram bot admins.
//
// It used to refuse to run unless Xray was up:
//
//	if !j.xrayService.IsXrayRunning() {
//		return
//	}
//
// That guard silenced the bot in exactly the situation it exists for. The report
// carries the server usage figures, the list of clients about to run out, and the
// core's own error text when it is down. An operator whose Xray has stopped needs
// all of that more than anyone, and the effect was the opposite: the bot simply
// went quiet, with no symptom anywhere else except that nothing arrived. Reported
// as "the bot works and manual backup works, but the automatic backup and report
// never come".
//
// The backup was collateral damage. SendReport also drives SendBackupToAdmins
// when tgBotBackup is on, and a database dump has no relationship to whether the
// proxy core is running. Manual backup kept working because its route calls
// SendBackupToAdmins directly and bypasses this job — which is the shape of the
// report: every manual path worked, every scheduled one did not.
//
// Nothing SendReport calls actually needs the core. The usage figures come from
// the status service, which reports the core's state rather than requiring it to
// be healthy; the exhausted-client list comes from the database; the backup is a
// database dump. So the gate is removed rather than narrowed, and the test beside
// this file fails if it comes back.
type StatsNotifyJob struct {
	sender reportSender
}

// NewStatsNotifyJob creates a new statistics notification job instance.
func NewStatsNotifyJob() *StatsNotifyJob {
	return newStatsNotifyJobWithSender(&tgbot.Tgbot{})
}

// newStatsNotifyJobWithSender builds the job around a caller-supplied sender, so
// tests can observe what Run does without a live bot.
func newStatsNotifyJobWithSender(sender reportSender) *StatsNotifyJob {
	return &StatsNotifyJob{sender: sender}
}

// Run sends the periodic report and, when enabled, the database backup.
func (j *StatsNotifyJob) Run() {
	if j.sender == nil {
		return
	}
	j.sender.SendReport()
}
