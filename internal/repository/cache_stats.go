package repository

import "time"

// CacheStats is a minimal placeholder cache status payload for admin APIs.
type CacheStats struct {
	Resource         string     `json:"resource"`
	Total            int        `json:"total"`
	ActiveCount      int        `json:"activeCount"`
	CacheImplemented bool       `json:"cacheImplemented"`
	LastRefreshedAt  *time.Time `json:"lastRefreshedAt,omitempty"`
	Note             string     `json:"note,omitempty"`
}
