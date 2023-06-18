package main

import (
	"io"
	"sync"
	"time"
)

type TunnelData struct {
	reader    io.Reader
	doneCH    chan struct{}
	CreatedAt time.Time
}

func NewTunnelData(reader io.Reader) *TunnelData {
	return &TunnelData{
		doneCH:    make(chan struct{}),
		CreatedAt: time.Now(),
		reader:    reader,
	}
}

func (td *TunnelData) Read(p []byte) (n int, err error) {
	return td.reader.Read(p)
}

func (td *TunnelData) Wait() {
	<-td.doneCH
}

func (td *TunnelData) Done() {
	close(td.doneCH)
}

type TunnelManager struct {
	tunnels map[string]*TunnelData
	lock    sync.RWMutex
}

func NewTunnelManager() *TunnelManager {
	return &TunnelManager{
		tunnels: make(map[string]*TunnelData),
	}
}

func (t *TunnelManager) AddTunnel(id string, tunnel *TunnelData) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.tunnels[id] = tunnel
}

func (t *TunnelManager) GetTunnel(id string) *TunnelData {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.tunnels[id]
}

func (t *TunnelManager) RemoveTunnel(id string) {
	t.lock.Lock()
	defer t.lock.Unlock()
	delete(t.tunnels, id)
}

func (t *TunnelManager) TunnelCount() int {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return len(t.tunnels)
}
