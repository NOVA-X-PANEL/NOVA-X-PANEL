package model

// OpenVPNUser represents an OpenVPN client account with traffic accounting,
// expiration time, and concurrent IP limit enforcement.
type OpenVPNUser struct {
	Id         int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name       string `json:"name" gorm:"uniqueIndex;not null"`
	Password   string `json:"password" gorm:"default:''"`
	Enable     bool   `json:"enable" gorm:"default:true"`
	TotalGB    int64  `json:"totalGB" gorm:"default:0"`    // 0 means unlimited; >0 limit in GB
	ExpiryTime int64  `json:"expiryTime" gorm:"default:0"` // 0 means unlimited; >0 timestamp in ms
	LimitIP    int    `json:"limitIp" gorm:"default:0"`    // 0 means unlimited; >0 max concurrent IPs
	Up         int64  `json:"up" gorm:"default:0"`         // cumulative bytes uploaded
	Down       int64  `json:"down" gorm:"default:0"`       // cumulative bytes downloaded
	CreatedAt  int64  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt  int64  `json:"updatedAt" gorm:"autoUpdateTime"`
}
