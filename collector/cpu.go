package collector

import (
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/load"
)

type CPUMetrics struct {
	TotalPercent float64
	PerCore      []float64
	Load1        float64
	Load5        float64
	Load15       float64
}

func CollectCPU() (CPUMetrics, error) {
	total, err := cpu.Percent(0, false)
	if err != nil {
		return CPUMetrics{}, err
	}
	perCore, _ := cpu.Percent(0, true)
	avg, _ := load.Avg()

	m := CPUMetrics{PerCore: perCore}
	if len(total) > 0 {
		m.TotalPercent = total[0]
	}
	if avg != nil {
		m.Load1 = avg.Load1
		m.Load5 = avg.Load5
		m.Load15 = avg.Load15
	}
	return m, nil
}
