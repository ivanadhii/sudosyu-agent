package collector

import "github.com/shirou/gopsutil/v4/mem"

type RAMMetrics struct {
	TotalGB     float64
	UsedGB      float64
	AvailableGB float64
	UsedPercent float64
	SwapTotalGB float64
	SwapUsedGB  float64
}

func CollectRAM() (RAMMetrics, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return RAMMetrics{}, err
	}
	s, _ := mem.SwapMemory()

	m := RAMMetrics{
		TotalGB:     toGB(v.Total),
		UsedGB:      toGB(v.Used),
		AvailableGB: toGB(v.Available),
		UsedPercent: v.UsedPercent,
	}
	if s != nil {
		m.SwapTotalGB = toGB(s.Total)
		m.SwapUsedGB = toGB(s.Used)
	}
	return m, nil
}

func toGB(bytes uint64) float64 {
	return float64(bytes) / (1024 * 1024 * 1024)
}
