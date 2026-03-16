package collector

import "github.com/shirou/gopsutil/v4/disk"

type DiskMetrics struct {
	MountPoint  string
	TotalGB     float64
	UsedGB      float64
	FreeGB      float64
	UsedPercent float64
}

func CollectDisks() ([]DiskMetrics, error) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}

	var result []DiskMetrics
	seen := map[string]bool{}
	for _, p := range partitions {
		if seen[p.Device] {
			continue
		}
		seen[p.Device] = true
		usage, err := disk.Usage(p.Mountpoint)
		if err != nil {
			continue
		}
		result = append(result, DiskMetrics{
			MountPoint:  p.Mountpoint,
			TotalGB:     toGB(usage.Total),
			UsedGB:      toGB(usage.Used),
			FreeGB:      toGB(usage.Free),
			UsedPercent: usage.UsedPercent,
		})
	}
	return result, nil
}
