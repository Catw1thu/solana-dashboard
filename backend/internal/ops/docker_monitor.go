package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

const runtimeDirUsageCacheTTL = 30 * time.Second

type DockerMonitor struct {
	enabled         bool
	socketPath      string
	containerPrefix string
	dataPath        string
	client          *http.Client
	dirCacheMu      sync.Mutex
	dirCache        runtimeDirUsageCache
}

type DockerMonitorConfig struct {
	Enabled         bool
	SocketPath      string
	ContainerPrefix string
	DataPath        string
}

type DockerSnapshot struct {
	Enabled         bool              `json:"enabled"`
	GeneratedAt     int64             `json:"generated_at"`
	ContainerPrefix string            `json:"container_prefix,omitempty"`
	Error           string            `json:"error,omitempty"`
	Totals          DockerTotals      `json:"totals"`
	Containers      []ContainerMetric `json:"containers"`
	Disk            DockerDiskUsage   `json:"disk"`
	RuntimeDirs     []RuntimeDirUsage `json:"runtime_dirs,omitempty"`
}

type DockerTotals struct {
	Containers       int     `json:"containers"`
	Running          int     `json:"running"`
	CPUPercent       float64 `json:"cpu_percent"`
	MemoryUsageBytes uint64  `json:"memory_usage_bytes"`
	MemoryLimitBytes uint64  `json:"memory_limit_bytes"`
	NetworkRxBytes   uint64  `json:"network_rx_bytes"`
	NetworkTxBytes   uint64  `json:"network_tx_bytes"`
	BlockReadBytes   uint64  `json:"block_read_bytes"`
	BlockWriteBytes  uint64  `json:"block_write_bytes"`
	WritableBytes    int64   `json:"writable_bytes"`
	RootFSBytes      int64   `json:"root_fs_bytes"`
}

type ContainerMetric struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Image            string  `json:"image"`
	State            string  `json:"state"`
	Status           string  `json:"status"`
	CPUPercent       float64 `json:"cpu_percent"`
	MemoryUsageBytes uint64  `json:"memory_usage_bytes"`
	MemoryLimitBytes uint64  `json:"memory_limit_bytes"`
	MemoryPercent    float64 `json:"memory_percent"`
	NetworkRxBytes   uint64  `json:"network_rx_bytes"`
	NetworkTxBytes   uint64  `json:"network_tx_bytes"`
	BlockReadBytes   uint64  `json:"block_read_bytes"`
	BlockWriteBytes  uint64  `json:"block_write_bytes"`
	WritableBytes    int64   `json:"writable_bytes"`
	RootFSBytes      int64   `json:"root_fs_bytes"`
}

type DockerDiskUsage struct {
	ImagesBytes      int64 `json:"images_bytes"`
	ContainersBytes  int64 `json:"containers_bytes"`
	VolumesBytes     int64 `json:"volumes_bytes"`
	BuildCacheBytes  int64 `json:"build_cache_bytes"`
	RuntimeDataBytes int64 `json:"runtime_data_bytes"`
}

type RuntimeDirUsage struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
	Error string `json:"error,omitempty"`
}

type runtimeDirUsageCache struct {
	expiresAt time.Time
	dirs      []RuntimeDirUsage
}

func NewDockerMonitor(cfg DockerMonitorConfig) *DockerMonitor {
	socketPath := cfg.SocketPath
	if socketPath == "" {
		socketPath = "/var/run/docker.sock"
	}
	prefix := cfg.ContainerPrefix
	if prefix == "" {
		prefix = "solana-dashboard-"
	}
	dataPath := cfg.DataPath
	if dataPath == "" {
		dataPath = "/ops-data"
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network string, addr string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}

	return &DockerMonitor{
		enabled:         cfg.Enabled,
		socketPath:      socketPath,
		containerPrefix: prefix,
		dataPath:        dataPath,
		client:          &http.Client{Transport: transport, Timeout: 5 * time.Second},
	}
}

func (m *DockerMonitor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snapshot, err := m.Snapshot(r.Context())
	if err != nil {
		snapshot.Enabled = m.enabled
		snapshot.GeneratedAt = time.Now().Unix()
		snapshot.ContainerPrefix = m.containerPrefix
		snapshot.Error = err.Error()
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	_ = json.NewEncoder(w).Encode(snapshot)
}

func (m *DockerMonitor) Snapshot(ctx context.Context) (DockerSnapshot, error) {
	snapshot := DockerSnapshot{
		Enabled:         m.enabled,
		GeneratedAt:     time.Now().Unix(),
		ContainerPrefix: m.containerPrefix,
	}
	snapshot.RuntimeDirs = m.runtimeDirUsage(ctx)
	for _, dir := range snapshot.RuntimeDirs {
		snapshot.Disk.RuntimeDataBytes += dir.Bytes
	}

	if !m.enabled {
		snapshot.Error = "docker monitoring is disabled"
		return snapshot, nil
	}

	containers, err := m.listContainers(ctx)
	if err != nil {
		return snapshot, err
	}

	for _, item := range containers {
		name := normalizeContainerName(item.Names)
		if !strings.HasPrefix(name, m.containerPrefix) {
			continue
		}

		metric := ContainerMetric{
			ID:     item.ID,
			Name:   name,
			Image:  item.Image,
			State:  item.State,
			Status: item.Status,
		}

		if item.State == "running" {
			stats, err := m.containerStats(ctx, item.ID)
			if err == nil {
				applyStats(&metric, stats)
			}
		}

		if inspect, err := m.containerInspect(ctx, item.ID); err == nil {
			metric.WritableBytes = inspect.SizeRw
			metric.RootFSBytes = inspect.SizeRootFs
		}

		snapshot.Containers = append(snapshot.Containers, metric)
	}

	sort.Slice(snapshot.Containers, func(i int, j int) bool {
		return snapshot.Containers[i].Name < snapshot.Containers[j].Name
	})

	for _, metric := range snapshot.Containers {
		snapshot.Totals.Containers += 1
		if metric.State == "running" {
			snapshot.Totals.Running += 1
		}
		snapshot.Totals.CPUPercent += metric.CPUPercent
		snapshot.Totals.MemoryUsageBytes += metric.MemoryUsageBytes
		snapshot.Totals.MemoryLimitBytes += metric.MemoryLimitBytes
		snapshot.Totals.NetworkRxBytes += metric.NetworkRxBytes
		snapshot.Totals.NetworkTxBytes += metric.NetworkTxBytes
		snapshot.Totals.BlockReadBytes += metric.BlockReadBytes
		snapshot.Totals.BlockWriteBytes += metric.BlockWriteBytes
		snapshot.Totals.WritableBytes += metric.WritableBytes
		snapshot.Totals.RootFSBytes += metric.RootFSBytes
	}

	if disk, err := m.systemDF(ctx); err == nil {
		disk.RuntimeDataBytes = snapshot.Disk.RuntimeDataBytes
		snapshot.Disk = disk
	}

	return snapshot, nil
}

func (m *DockerMonitor) runtimeDirUsage(ctx context.Context) []RuntimeDirUsage {
	if m.dataPath == "" {
		return nil
	}

	now := time.Now()
	m.dirCacheMu.Lock()
	if now.Before(m.dirCache.expiresAt) {
		dirs := cloneRuntimeDirs(m.dirCache.dirs)
		m.dirCacheMu.Unlock()
		return dirs
	}
	m.dirCacheMu.Unlock()

	dirs := scanRuntimeDirs(ctx, m.dataPath)

	m.dirCacheMu.Lock()
	m.dirCache = runtimeDirUsageCache{
		expiresAt: now.Add(runtimeDirUsageCacheTTL),
		dirs:      cloneRuntimeDirs(dirs),
	}
	m.dirCacheMu.Unlock()

	return dirs
}

func scanRuntimeDirs(ctx context.Context, dataPath string) []RuntimeDirUsage {
	entries, err := os.ReadDir(dataPath)
	if err != nil {
		return []RuntimeDirUsage{{
			Name:  filepath.Base(dataPath),
			Path:  dataPath,
			Error: err.Error(),
		}}
	}

	dirs := make([]RuntimeDirUsage, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(dataPath, entry.Name())
		bytes, err := directoryDiskUsage(ctx, path)
		usage := RuntimeDirUsage{
			Name:  entry.Name(),
			Path:  path,
			Bytes: bytes,
		}
		if err != nil {
			usage.Error = err.Error()
		}
		dirs = append(dirs, usage)
	}

	if len(dirs) == 0 {
		bytes, err := directoryDiskUsage(ctx, dataPath)
		usage := RuntimeDirUsage{
			Name:  filepath.Base(dataPath),
			Path:  dataPath,
			Bytes: bytes,
		}
		if err != nil {
			usage.Error = err.Error()
		}
		dirs = append(dirs, usage)
	}

	sort.Slice(dirs, func(i int, j int) bool {
		return dirs[i].Name < dirs[j].Name
	})
	return dirs
}

func directoryDiskUsage(ctx context.Context, root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		total += diskUsageBytes(info)
		return nil
	})
	return total, err
}

func diskUsageBytes(info os.FileInfo) int64 {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat.Blocks > 0 {
		return stat.Blocks * 512
	}
	return info.Size()
}

func cloneRuntimeDirs(dirs []RuntimeDirUsage) []RuntimeDirUsage {
	if len(dirs) == 0 {
		return nil
	}
	out := make([]RuntimeDirUsage, len(dirs))
	copy(out, dirs)
	return out
}

func (m *DockerMonitor) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker"+path, nil)
	if err != nil {
		return err
	}
	res, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("docker api %s returned status %d", path, res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func (m *DockerMonitor) listContainers(ctx context.Context) ([]dockerContainer, error) {
	var containers []dockerContainer
	query := url.Values{}
	query.Set("all", "true")
	if err := m.getJSON(ctx, "/containers/json?"+query.Encode(), &containers); err != nil {
		return nil, err
	}
	return containers, nil
}

func (m *DockerMonitor) containerStats(ctx context.Context, id string) (dockerStats, error) {
	var stats dockerStats
	err := m.getJSON(ctx, "/containers/"+id+"/stats?stream=false", &stats)
	return stats, err
}

func (m *DockerMonitor) containerInspect(ctx context.Context, id string) (dockerInspect, error) {
	var inspect dockerInspect
	err := m.getJSON(ctx, "/containers/"+id+"/json?size=true", &inspect)
	return inspect, err
}

func (m *DockerMonitor) systemDF(ctx context.Context) (DockerDiskUsage, error) {
	var df dockerSystemDF
	if err := m.getJSON(ctx, "/system/df", &df); err != nil {
		return DockerDiskUsage{}, err
	}
	usage := DockerDiskUsage{}
	for _, image := range df.Images {
		usage.ImagesBytes += image.Size
	}
	for _, container := range df.Containers {
		usage.ContainersBytes += container.SizeRw
	}
	for _, volume := range df.Volumes {
		usage.VolumesBytes += volume.UsageData.Size
	}
	for _, cache := range df.BuildCache {
		usage.BuildCacheBytes += cache.Size
	}
	return usage, nil
}

func applyStats(metric *ContainerMetric, stats dockerStats) {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemCPUUsage - stats.PreCPUStats.SystemCPUUsage)
	onlineCPUs := float64(stats.CPUStats.OnlineCPUs)
	if onlineCPUs == 0 {
		onlineCPUs = float64(len(stats.CPUStats.CPUUsage.PercpuUsage))
	}
	if systemDelta > 0 && cpuDelta > 0 && onlineCPUs > 0 {
		metric.CPUPercent = (cpuDelta / systemDelta) * onlineCPUs * 100
	}

	metric.MemoryUsageBytes = stats.MemoryStats.Usage
	metric.MemoryLimitBytes = stats.MemoryStats.Limit
	if metric.MemoryLimitBytes > 0 {
		metric.MemoryPercent = float64(metric.MemoryUsageBytes) / float64(metric.MemoryLimitBytes) * 100
	}
	for _, network := range stats.Networks {
		metric.NetworkRxBytes += network.RxBytes
		metric.NetworkTxBytes += network.TxBytes
	}
	for _, item := range stats.BlkioStats.IOServiceBytesRecursive {
		switch strings.ToLower(item.Op) {
		case "read":
			metric.BlockReadBytes += item.Value
		case "write":
			metric.BlockWriteBytes += item.Value
		}
	}
}

func normalizeContainerName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.TrimPrefix(names[0], "/")
}

type dockerContainer struct {
	ID     string   `json:"Id"`
	Names  []string `json:"Names"`
	Image  string   `json:"Image"`
	State  string   `json:"State"`
	Status string   `json:"Status"`
}

type dockerInspect struct {
	SizeRw     int64 `json:"SizeRw"`
	SizeRootFs int64 `json:"SizeRootFs"`
}

type dockerStats struct {
	CPUStats    dockerCPUStats    `json:"cpu_stats"`
	PreCPUStats dockerCPUStats    `json:"precpu_stats"`
	MemoryStats dockerMemoryStats `json:"memory_stats"`
	Networks    map[string]struct {
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

type dockerCPUStats struct {
	OnlineCPUs     uint64 `json:"online_cpus"`
	SystemCPUUsage uint64 `json:"system_cpu_usage"`
	CPUUsage       struct {
		TotalUsage  uint64   `json:"total_usage"`
		PercpuUsage []uint64 `json:"percpu_usage"`
	} `json:"cpu_usage"`
}

type dockerMemoryStats struct {
	Usage uint64 `json:"usage"`
	Limit uint64 `json:"limit"`
}

type dockerSystemDF struct {
	Images []struct {
		Size int64 `json:"Size"`
	} `json:"Images"`
	Containers []struct {
		SizeRw int64 `json:"SizeRw"`
	} `json:"Containers"`
	Volumes []struct {
		UsageData struct {
			Size int64 `json:"Size"`
		} `json:"UsageData"`
	} `json:"Volumes"`
	BuildCache []struct {
		Size int64 `json:"Size"`
	} `json:"BuildCache"`
}
