package collector

import (
	"time"

	"github.com/shirou/gopsutil/v4/disk"
)

type DiskIOMetrics struct {
	Device           string
	ReadBytesPerSec  float64
	WriteBytesPerSec float64
	ReadIOPS         float64
	WriteIOPS        float64
}

type DiskIOCollector struct {
	prev     map[string]disk.IOCountersStat
	prevTime time.Time
}

func NewDiskIOCollector() *DiskIOCollector {
	return &DiskIOCollector{}
}

func (c *DiskIOCollector) Collect() ([]DiskIOMetrics, error) {
	counters, err := disk.IOCounters()
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

	var result []DiskIOMetrics
	for name, cur := range counters {
		prev, ok := c.prev[name]
		if !ok {
			continue
		}
		result = append(result, DiskIOMetrics{
			Device:           name,
			ReadBytesPerSec:  float64(cur.ReadBytes-prev.ReadBytes) / elapsed,
			WriteBytesPerSec: float64(cur.WriteBytes-prev.WriteBytes) / elapsed,
			ReadIOPS:         float64(cur.ReadCount-prev.ReadCount) / elapsed,
			WriteIOPS:        float64(cur.WriteCount-prev.WriteCount) / elapsed,
		})
	}

	c.prev = counters
	c.prevTime = now
	return result, nil
}
