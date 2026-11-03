package model

// OpenVPNSession tracks active OpenVPN client sessions to calculate traffic deltas
// and survive panel restarts without double-counting.
type OpenVPNSession struct {
	Id          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string `json:"name" gorm:"not null;index"`
	RealIp      string `json:"realIp" gorm:"column:real_ip;not null"`
	RealPort    string `json:"realPort" gorm:"column:real_port;not null"`
	VirtualIp   string `json:"virtualIp" gorm:"column:virtual_ip;default:''"`
	LastRx      int64  `json:"lastRx" gorm:"column:last_rx;default:0"`
	LastTx      int64  `json:"lastTx" gorm:"column:last_tx;default:0"`
	ConnectedAt int64  `json:"connectedAt" gorm:"column:connected_at;default:0"`
	UpdatedAt   int64  `json:"updatedAt" gorm:"column:updated_at;default:0"`
}
