package sender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type MetricRow struct {
	Type  string  `json:"type"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

type ContainerSnapshot struct {
	ContainerID  string   `json:"containerId"`
	Name         string   `json:"name"`
	Image        string   `json:"image"`
	Status       string   `json:"status"`
	Uptime       string   `json:"uptime"`
	RestartCount int      `json:"restartCount"`
	Ports        []string `json:"ports"`
	CPUPercent   *float64 `json:"cpuPercent"`
	MemMB        *float64 `json:"memMB"`
	MemPercent   *float64 `json:"memPercent"`
	NetIn        *float64 `json:"netIn"`
	NetOut       *float64 `json:"netOut"`
	BlockRead    *float64 `json:"blockRead"`
	BlockWrite   *float64 `json:"blockWrite"`
}

type DockerDFSnapshot struct {
	ImagesSize            int64 `json:"imagesSize"`
	ImagesReclaimable     int64 `json:"imagesReclaimable"`
	ContainersSize        int64 `json:"containersSize"`
	VolumesSize           int64 `json:"volumesSize"`
	VolumesReclaimable    int64 `json:"volumesReclaimable"`
	BuildCacheSize        int64 `json:"buildCacheSize"`
	BuildCacheReclaimable int64 `json:"buildCacheReclaimable"`
}

type Payload struct {
	ServerName string              `json:"serverName"`
	Timestamp  time.Time           `json:"timestamp"`
	Metrics    []MetricRow         `json:"metrics"`
	Containers []ContainerSnapshot `json:"containers"`
	DockerDF   *DockerDFSnapshot   `json:"dockerDf,omitempty"`
}

type Sender struct {
	backendURL string
	apiKey     string
	client     *http.Client
	buffer     []Payload
}

func New(backendURL, apiKey string) *Sender {
	return &Sender{
		backendURL: backendURL,
		apiKey:     apiKey,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *Sender) Send(p Payload) error {
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", s.backendURL+"/api/v1/metrics", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		// Buffer for retry
		s.buffer = append(s.buffer, p)
		return fmt.Errorf("send failed (buffered): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		s.buffer = append(s.buffer, p)
		return fmt.Errorf("backend returned %d (buffered)", resp.StatusCode)
	}

	// Flush buffer if send succeeded
	if len(s.buffer) > 0 {
		s.flushBuffer()
	}

	return nil
}

func (s *Sender) flushBuffer() {
	var failed []Payload
	for _, p := range s.buffer {
		body, _ := json.Marshal(p)
		req, err := http.NewRequest("POST", s.backendURL+"/api/v1/metrics", bytes.NewReader(body))
		if err != nil {
			failed = append(failed, p)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", s.apiKey)
		resp, err := s.client.Do(req)
		if err != nil || resp.StatusCode >= 300 {
			failed = append(failed, p)
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}
		resp.Body.Close()
	}
	s.buffer = failed
}
