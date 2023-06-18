package main

import (
	"errors"
	"io"
	"sync"
	"time"
)

const TunnelTTL = 17 * time.Minute // prime number of minutes or it won't work

var ErrStreamSizeExceeded = errors.New("stream size exceeded")

type TunnelData struct {
	reader    io.Reader
	dataRed   int
	doneCH    chan struct{}
	CreatedAt time.Time
	lock      sync.Mutex
}

func NewTunnelData(reader io.Reader) *TunnelData {
	return &TunnelData{
		doneCH:    make(chan struct{}),
		CreatedAt: time.Now(),
		reader:    reader,
	}
}

func (td *TunnelData) Read(p []byte) (int, error) {
	td.lock.Lock()
	defer td.lock.Unlock()

	n, err := td.reader.Read(p)
	td.dataRed += n
	if td.dataRed > MaxStreamSize {
		return 0, ErrStreamSizeExceeded
	}
	return n, err
}

func (td *TunnelData) Wait() {
	<-td.doneCH
}

func (td *TunnelData) Done() {
	td.lock.Lock()
	defer td.lock.Unlock()
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
	t.removeTunnel(id)
}

func (t *TunnelManager) removeTunnel(id string) {
	delete(t.tunnels, id)
}

func (t *TunnelManager) CleanUp() uint {
	t.lock.Lock()
	defer t.lock.Unlock()

	cleanedUpCount := uint(0)

	for id, tunnel := range t.tunnels {
		if time.Since(tunnel.CreatedAt) > TunnelTTL {
			tunnel.Done()
			t.removeTunnel(id)
			cleanedUpCount++
		}
	}

	return cleanedUpCount
}

func (t *TunnelManager) TunnelCount() int {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return len(t.tunnels)
}
