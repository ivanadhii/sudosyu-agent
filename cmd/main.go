package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sudosyu/agent/collector"
	"github.com/sudosyu/agent/config"
	"github.com/sudosyu/agent/sender"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	if cfg.APIKey == "" {
		log.Fatal("API_KEY is required")
	}
	if cfg.ServerName == "" {
		hostname, _ := os.Hostname()
		cfg.ServerName = hostname
	}

	log.Printf("Sudosyu Agent starting — server: %s, backend: %s, interval: %ds",
		cfg.ServerName, cfg.BackendURL, cfg.IntervalSeconds)

	s := sender.New(cfg.BackendURL, cfg.APIKey)
	diskIOCollector := collector.NewDiskIOCollector()
	netCollector := collector.NewNetworkCollector()

	var dockerCollector *collector.DockerCollector
	if _, err := os.Stat(cfg.DockerSocket); err == nil {
		dockerCollector = collector.NewDockerCollector(cfg.DockerSocket)
		log.Printf("Docker socket found at %s — container monitoring enabled", cfg.DockerSocket)
	} else {
		log.Printf("Docker socket not found at %s — container monitoring disabled", cfg.DockerSocket)
	}

	ticker := time.NewTicker(time.Duration(cfg.IntervalSeconds) * time.Second)
	defer ticker.Stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Collect and send immediately on start
	collect(s, cfg.ServerName, diskIOCollector, netCollector, dockerCollector)

	for {
		select {
		case <-ticker.C:
			collect(s, cfg.ServerName, diskIOCollector, netCollector, dockerCollector)
		case <-quit:
			log.Println("Sudosyu Agent stopped.")
			return
		}
	}
}

func collect(
	s *sender.Sender,
	serverName string,
	diskIO *collector.DiskIOCollector,
	net *collector.NetworkCollector,
	docker *collector.DockerCollector,
) {
	now := time.Now()
	var metrics []sender.MetricRow

	// CPU
	if cpu, err := collector.CollectCPU(); err == nil {
		metrics = append(metrics, sender.MetricRow{Type: "cpu", Name: "total", Value: cpu.TotalPercent})
		metrics = append(metrics, sender.MetricRow{Type: "cpu", Name: "load1", Value: cpu.Load1})
		metrics = append(metrics, sender.MetricRow{Type: "cpu", Name: "load5", Value: cpu.Load5})
		metrics = append(metrics, sender.MetricRow{Type: "cpu", Name: "load15", Value: cpu.Load15})
		for i, v := range cpu.PerCore {
			metrics = append(metrics, sender.MetricRow{
				Type: "cpu", Name: fmt.Sprintf("core_%d", i), Value: v,
			})
		}
	}

	// RAM
	if ram, err := collector.CollectRAM(); err == nil {
		metrics = append(metrics,
			sender.MetricRow{Type: "ram", Name: "used_percent", Value: ram.UsedPercent},
			sender.MetricRow{Type: "ram", Name: "used_gb", Value: ram.UsedGB},
			sender.MetricRow{Type: "ram", Name: "total_gb", Value: ram.TotalGB},
			sender.MetricRow{Type: "ram", Name: "available_gb", Value: ram.AvailableGB},
			sender.MetricRow{Type: "ram", Name: "swap_total_gb", Value: ram.SwapTotalGB},
			sender.MetricRow{Type: "ram", Name: "swap_used_gb", Value: ram.SwapUsedGB},
		)
	}

	// Disk
	if disks, err := collector.CollectDisks(); err == nil {
		for i, d := range disks {
			suffix := d.MountPoint
			// Use "disk" for first/root mount for the summary endpoint
			if i == 0 {
				metrics = append(metrics, sender.MetricRow{Type: "disk", Name: "used_percent", Value: d.UsedPercent})
			}
			metrics = append(metrics,
				sender.MetricRow{Type: "disk", Name: suffix + "_used_percent", Value: d.UsedPercent},
				sender.MetricRow{Type: "disk", Name: suffix + "_used_gb", Value: d.UsedGB},
				sender.MetricRow{Type: "disk", Name: suffix + "_total_gb", Value: d.TotalGB},
				sender.MetricRow{Type: "disk", Name: suffix + "_free_gb", Value: d.FreeGB},
			)
		}
	}

	// Disk I/O
	if ios, err := diskIO.Collect(); err == nil && ios != nil {
		for _, io := range ios {
			metrics = append(metrics,
				sender.MetricRow{Type: "disk_io", Name: "read_bytes_per_sec", Value: io.ReadBytesPerSec},
				sender.MetricRow{Type: "disk_io", Name: "write_bytes_per_sec", Value: io.WriteBytesPerSec},
				sender.MetricRow{Type: "disk_io", Name: "read_iops", Value: io.ReadIOPS},
				sender.MetricRow{Type: "disk_io", Name: "write_iops", Value: io.WriteIOPS},
			)
		}
	}

	// Network
	if nets, err := net.Collect(); err == nil && nets != nil {
		for _, n := range nets {
			metrics = append(metrics,
				sender.MetricRow{Type: "network", Name: "bytes_recv_per_sec", Value: n.BytesRecvPerSec},
				sender.MetricRow{Type: "network", Name: "bytes_sent_per_sec", Value: n.BytesSentPerSec},
				sender.MetricRow{Type: "network", Name: "packets_recv_per_sec", Value: n.PacketsRecvPerSec},
				sender.MetricRow{Type: "network", Name: "packets_sent_per_sec", Value: n.PacketsSentPerSec},
			)
		}
	}

	payload := sender.Payload{
		ServerName: serverName,
		Timestamp:  now,
		Metrics:    metrics,
	}

	// Docker
	if docker != nil {
		if containers, err := docker.CollectContainers(); err == nil {
			for _, c := range containers {
				uptime := ""
				payload.Containers = append(payload.Containers, sender.ContainerSnapshot{
					ContainerID:  c.ID,
					Name:         c.Name,
					Image:        c.Image,
					Status:       c.Status,
					Uptime:       uptime,
					RestartCount: c.RestartCount,
					Ports:        c.Ports,
					CPUPercent:   c.CPUPercent,
					MemMB:        c.MemMB,
					MemPercent:   c.MemPercent,
					NetIn:        c.NetIn,
					NetOut:       c.NetOut,
					BlockRead:    c.BlockRead,
					BlockWrite:   c.BlockWrite,
				})
			}
		}

		if df, err := docker.CollectDF(); err == nil && df != nil {
			payload.DockerDF = &sender.DockerDFSnapshot{
				ImagesSize:            df.ImagesSize,
				ImagesReclaimable:     df.ImagesReclaimable,
				ContainersSize:        df.ContainersSize,
				VolumesSize:           df.VolumesSize,
				VolumesReclaimable:    df.VolumesReclaimable,
				BuildCacheSize:        df.BuildCacheSize,
				BuildCacheReclaimable: df.BuildCacheReclaimable,
			}
		}
	}

	if err := s.Send(payload); err != nil {
		log.Printf("send error: %v", err)
	} else {
		log.Printf("sent %d metrics, %d containers", len(metrics), len(payload.Containers))
	}
}
