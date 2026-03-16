package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type DockerContainer struct {
	ID           string
	Name         string
	Image        string
	Status       string
	Uptime       string
	RestartCount int
	Ports        []string
	CreatedAt    string

	// Resource stats (only for running containers)
	CPUPercent *float64
	MemMB      *float64
	MemPercent *float64
	NetIn      *float64
	NetOut     *float64
	BlockRead  *float64
	BlockWrite *float64
}

type DockerDF struct {
	ImagesSize            int64
	ImagesReclaimable     int64
	ContainersSize        int64
	VolumesSize           int64
	VolumesReclaimable    int64
	BuildCacheSize        int64
	BuildCacheReclaimable int64
}

type DockerCollector struct {
	client     *http.Client
	socketPath string
}

func NewDockerCollector(socketPath string) *DockerCollector {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		},
	}
	return &DockerCollector{
		client:     &http.Client{Transport: transport, Timeout: 10 * time.Second},
		socketPath: socketPath,
	}
}

func (d *DockerCollector) get(path string) ([]byte, error) {
	resp, err := d.client.Get("http://localhost" + path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (d *DockerCollector) CollectContainers() ([]DockerContainer, error) {
	body, err := d.get("/containers/json?all=true")
	if err != nil {
		return nil, err
	}

	var raw []struct {
		ID     string `json:"Id"`
		Names  []string
		Image  string
		State  string
		Status string // human-readable e.g. "Up 2 hours"
		Ports  []struct {
			IP          string
			PrivatePort int
			PublicPort  int
			Type        string
		}
		Created    int64
		HostConfig struct {
			RestartPolicy struct {
				MaximumRetryCount int
			}
		}
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	containers := make([]DockerContainer, len(raw))
	var wg sync.WaitGroup

	for i, r := range raw {
		name := r.ID[:12]
		if len(r.Names) > 0 {
			name = strings.TrimPrefix(r.Names[0], "/")
		}

		var ports []string
		for _, p := range r.Ports {
			if p.PublicPort > 0 {
				ports = append(ports, fmt.Sprintf("%d:%d/%s", p.PublicPort, p.PrivatePort, p.Type))
			}
		}

		containers[i] = DockerContainer{
			ID:           r.ID[:12],
			Name:         name,
			Image:        r.Image,
			Status:       strings.ToLower(r.State),
			Uptime:       r.Status,
			RestartCount: r.HostConfig.RestartPolicy.MaximumRetryCount,
			Ports:        ports,
			CreatedAt:    time.Unix(r.Created, 0).UTC().Format(time.RFC3339),
		}

		if containers[i].Status == "running" {
			wg.Add(1)
			go func(idx int, id string) {
				defer wg.Done()
				d.fillStats(&containers[idx], id)
			}(i, r.ID)
		}
	}

	wg.Wait()
	return containers, nil
}

func (d *DockerCollector) fillStats(c *DockerContainer, containerID string) {
	body, err := d.get("/containers/" + containerID + "/stats?stream=false")
	if err != nil {
		return
	}

	var stats struct {
		CPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage uint64 `json:"system_cpu_usage"`
			OnlineCPUs     int    `json:"online_cpus"`
		} `json:"cpu_stats"`
		PreCPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage uint64 `json:"system_cpu_usage"`
		} `json:"precpu_stats"`
		MemoryStats struct {
			Usage uint64 `json:"usage"`
			Limit uint64 `json:"limit"`
			Stats struct {
				Cache uint64 `json:"cache"`
			} `json:"stats"`
		} `json:"memory_stats"`
		Networks map[string]struct {
			RxBytes uint64 `json:"rx_bytes"`
			TxBytes uint64 `json:"tx_bytes"`
		} `json:"networks"`
		BlkioStats struct {
			IOServiceBytesRecursive []struct {
				Op    string `json:"op"`
				Value uint64 `json:"value"`
			} `json:"io_service_bytes_recursive"`
		} `json:"blkio_stats"`
	}
	if err := json.Unmarshal(body, &stats); err != nil {
		return
	}

	// CPU %
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(stats.CPUStats.SystemCPUUsage - stats.PreCPUStats.SystemCPUUsage)
	numCPU := stats.CPUStats.OnlineCPUs
	if numCPU == 0 {
		numCPU = 1
	}
	var cpuPct float64
	if sysDelta > 0 {
		cpuPct = (cpuDelta / sysDelta) * float64(numCPU) * 100.0
	}
	c.CPUPercent = &cpuPct

	// Memory
	memUsage := stats.MemoryStats.Usage - stats.MemoryStats.Stats.Cache
	memLimit := stats.MemoryStats.Limit
	memMB := float64(memUsage) / (1024 * 1024)
	c.MemMB = &memMB
	if memLimit > 0 {
		memPct := float64(memUsage) / float64(memLimit) * 100.0
		c.MemPercent = &memPct
	}

	// Network
	var rxTotal, txTotal uint64
	for _, iface := range stats.Networks {
		rxTotal += iface.RxBytes
		txTotal += iface.TxBytes
	}
	netIn := float64(rxTotal)
	netOut := float64(txTotal)
	c.NetIn = &netIn
	c.NetOut = &netOut

	// Block I/O
	var blockRead, blockWrite float64
	for _, entry := range stats.BlkioStats.IOServiceBytesRecursive {
		switch entry.Op {
		case "Read":
			blockRead += float64(entry.Value)
		case "Write":
			blockWrite += float64(entry.Value)
		}
	}
	c.BlockRead = &blockRead
	c.BlockWrite = &blockWrite
}

func (d *DockerCollector) CollectDF() (*DockerDF, error) {
	body, err := d.get("/system/df")
	if err != nil {
		return nil, err
	}

	var raw struct {
		Images []struct {
			Size       int64 `json:"Size"`
			SharedSize int64 `json:"SharedSize"`
		}
		Containers []struct {
			SizeRootFs int64 `json:"SizeRootFs"`
		}
		Volumes []struct {
			UsageData struct {
				Size     int64 `json:"Size"`
				RefCount int   `json:"RefCount"`
			} `json:"UsageData"`
		}
		BuildCache []struct {
			Size  int64 `json:"Size"`
			InUse bool  `json:"InUse"`
		}
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	df := &DockerDF{}
	for _, img := range raw.Images {
		df.ImagesSize += img.Size
	}
	for _, c := range raw.Containers {
		df.ContainersSize += c.SizeRootFs
	}
	for _, v := range raw.Volumes {
		df.VolumesSize += v.UsageData.Size
		if v.UsageData.RefCount == 0 {
			df.VolumesReclaimable += v.UsageData.Size
		}
	}
	for _, bc := range raw.BuildCache {
		df.BuildCacheSize += bc.Size
		if !bc.InUse {
			df.BuildCacheReclaimable += bc.Size
		}
	}
	return df, nil
}
