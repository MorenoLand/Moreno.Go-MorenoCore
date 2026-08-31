package world

import "sync"

type LFGManager struct {
	mu      sync.RWMutex
	enabled bool
	solo    bool
}

func NewLFGManager(enabled bool) *LFGManager {
	return &LFGManager{enabled: enabled}
}

func (m *LFGManager) OnLogin() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.enabled && !m.solo {
		m.solo = true
	}
}

func (m *LFGManager) Enabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

func (m *LFGManager) Solo() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.solo
}

func (m *LFGManager) ToggleSolo() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.solo = !m.solo
	return m.solo
}

func (m *LFGManager) RequiresFullGroup(players, groupSize int) bool {
	if players < 0 || groupSize < 0 {
		return true
	}
	return !m.Solo() && players != groupSize
}
