package circuitbreaker

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestVendorTypeBreaker_InitiallyClosed(t *testing.T) {
	m := NewVendorTypeBreakerManager()
	assert.False(t, m.IsOpen(1, "claude"))
	assert.Nil(t, m.GetState(1, "claude"))
}

func TestVendorTypeBreaker_TriggerOpen(t *testing.T) {
	m := NewVendorTypeBreakerManager()

	m.TriggerOpen(1, "claude")
	assert.True(t, m.IsOpen(1, "claude"))

	s := m.GetState(1, "claude")
	assert.NotNil(t, s)
	assert.Equal(t, "open", s.State)
	assert.Equal(t, 1, s.VendorID)
	assert.Equal(t, "claude", s.ProviderType)
}

func TestVendorTypeBreaker_AutoRecovery(t *testing.T) {
	m := NewVendorTypeBreakerManager()

	m.TriggerOpen(1, "claude", 50*time.Millisecond)
	assert.True(t, m.IsOpen(1, "claude"))

	// Advance time past open duration.
	m.mu.Lock()
	m.nowFunc = func() time.Time { return time.Now().Add(100 * time.Millisecond) }
	m.mu.Unlock()

	assert.False(t, m.IsOpen(1, "claude"))
}

func TestVendorTypeBreaker_ManualOpen(t *testing.T) {
	m := NewVendorTypeBreakerManager()

	m.SetManualOpen(1, "claude", true)
	assert.True(t, m.IsOpen(1, "claude"))

	s := m.GetState(1, "claude")
	assert.NotNil(t, s)
	assert.True(t, s.ManualOpen)

	// Manual open should not auto-recover.
	m.mu.Lock()
	m.nowFunc = func() time.Time { return time.Now().Add(24 * time.Hour) }
	m.mu.Unlock()
	assert.True(t, m.IsOpen(1, "claude"))

	// Close manually.
	m.SetManualOpen(1, "claude", false)
	assert.False(t, m.IsOpen(1, "claude"))
}

func TestVendorTypeBreaker_TriggerIgnoredWhenManualOpen(t *testing.T) {
	m := NewVendorTypeBreakerManager()

	m.SetManualOpen(1, "claude", true)

	// TriggerOpen should be ignored when manually open.
	m.TriggerOpen(1, "claude", 50*time.Millisecond)
	s := m.GetState(1, "claude")
	assert.True(t, s.ManualOpen)
}

func TestVendorTypeBreaker_CustomDuration(t *testing.T) {
	m := NewVendorTypeBreakerManager()

	m.TriggerOpen(1, "claude", 200*time.Millisecond)

	// Still open at 100ms.
	m.mu.Lock()
	m.nowFunc = func() time.Time { return time.Now().Add(100 * time.Millisecond) }
	m.mu.Unlock()
	assert.True(t, m.IsOpen(1, "claude"))

	// Recovered at 300ms.
	m.mu.Lock()
	m.nowFunc = func() time.Time { return time.Now().Add(300 * time.Millisecond) }
	m.mu.Unlock()
	assert.False(t, m.IsOpen(1, "claude"))
}

func TestVendorTypeBreaker_Reset(t *testing.T) {
	m := NewVendorTypeBreakerManager()

	m.TriggerOpen(1, "claude")
	assert.True(t, m.IsOpen(1, "claude"))

	m.Reset(1, "claude")
	assert.False(t, m.IsOpen(1, "claude"))
	assert.Nil(t, m.GetState(1, "claude"))
}

func TestVendorTypeBreaker_ResetAll(t *testing.T) {
	m := NewVendorTypeBreakerManager()

	m.TriggerOpen(1, "claude")
	m.TriggerOpen(2, "openai")
	m.ResetAll()

	assert.False(t, m.IsOpen(1, "claude"))
	assert.False(t, m.IsOpen(2, "openai"))
}

func TestVendorTypeBreaker_DifferentKeysIndependent(t *testing.T) {
	m := NewVendorTypeBreakerManager()

	m.TriggerOpen(1, "claude")
	assert.True(t, m.IsOpen(1, "claude"))
	assert.False(t, m.IsOpen(1, "openai"))
	assert.False(t, m.IsOpen(2, "claude"))
}

func TestVendorTypeBreaker_GetState_ShowsClosedAfterExpiry(t *testing.T) {
	m := NewVendorTypeBreakerManager()

	m.TriggerOpen(1, "claude", 50*time.Millisecond)

	m.mu.Lock()
	m.nowFunc = func() time.Time { return time.Now().Add(100 * time.Millisecond) }
	m.mu.Unlock()

	s := m.GetState(1, "claude")
	assert.NotNil(t, s)
	assert.Equal(t, "closed", s.State)
}

func TestVendorTypeBreaker_ConcurrentAccess(t *testing.T) {
	m := NewVendorTypeBreakerManager()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			vid := id%3 + 1
			pt := "claude"
			if id%2 == 0 {
				pt = "openai"
			}
			for j := 0; j < 100; j++ {
				m.TriggerOpen(vid, pt, 50*time.Millisecond)
				m.IsOpen(vid, pt)
				m.GetState(vid, pt)
				if j%10 == 0 {
					m.Reset(vid, pt)
				}
			}
		}(i)
	}
	wg.Wait()
}
