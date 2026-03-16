package collector

import (
	"time"

	"github.com/shirou/gopsutil/v4/net"
)

type NetworkMetrics struct {
	Interface         string
	BytesSentPerSec   float64
	BytesRecvPerSec   float64
	PacketsSentPerSec float64
	PacketsRecvPerSec float64
}

type NetworkCollector struct {
	prev     []net.IOCountersStat
	prevTime time.Time
}

func NewNetworkCollector() *NetworkCollector {
	return &NetworkCollector{}
}

func (c *NetworkCollector) Collect() ([]NetworkMetrics, error) {
	counters, err := net.IOCounters(true)
	if err != nil {
		return nil, err
	}
	now := time.Now()

	if c.prev == nil {
		c.prev = counters
		c.prevTime = now
		return nil, nil
	}

	elapsed := now.Sub(c.prevTime).Seconds()
	if elapsed <= 0 {
		elapsed = 1
	}

	prevMap := map[string]net.IOCountersStat{}
	for _, p := range c.prev {
		prevMap[p.Name] = p
	}

	var result []NetworkMetrics
	for _, cur := range counters {
		prev, ok := prevMap[cur.Name]
		if !ok {
			continue
		}
		// Skip loopback
		if cur.Name == "lo" {
			continue
		}
		result = append(result, NetworkMetrics{
			Interface:         cur.Name,
			BytesSentPerSec:   float64(cur.BytesSent-prev.BytesSent) / elapsed,
			BytesRecvPerSec:   float64(cur.BytesRecv-prev.BytesRecv) / elapsed,
			PacketsSentPerSec: float64(cur.PacketsSent-prev.PacketsSent) / elapsed,
			PacketsRecvPerSec: float64(cur.PacketsRecv-prev.PacketsRecv) / elapsed,
		})
	}

	c.prev = counters
	c.prevTime = now
	return result, nil
}
