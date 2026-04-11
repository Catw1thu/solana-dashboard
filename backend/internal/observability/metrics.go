package observability

import (
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var defaultRegistry = NewRegistry()

type HistogramSnapshot struct {
	Count        uint64    `json:"count"`
	Sum          float64   `json:"sum"`
	Average      float64   `json:"average"`
	Bounds       []float64 `json:"bounds"`
	BucketCounts []uint64  `json:"bucket_counts"`
}

type Snapshot struct {
	UptimeSeconds int64                        `json:"uptime_seconds"`
	Counters      map[string]int64             `json:"counters"`
	Gauges        map[string]int64             `json:"gauges"`
	Histograms    map[string]HistogramSnapshot `json:"histograms"`
}

type Registry struct {
	startedAt time.Time

	mu         sync.RWMutex
	counters   map[string]*atomic.Int64
	gauges     map[string]*atomic.Int64
	histograms map[string]*histogram
}

type histogram struct {
	mu     sync.Mutex
	bounds []float64
	counts []uint64
	count  uint64
	sum    float64
}

func NewRegistry() *Registry {
	return &Registry{
		startedAt:  time.Now(),
		counters:   make(map[string]*atomic.Int64),
		gauges:     make(map[string]*atomic.Int64),
		histograms: map[string]*histogram{},
	}
}

func Default() *Registry {
	return defaultRegistry
}

func (r *Registry) IncCounter(name string, delta int64) {
	if delta == 0 {
		return
	}
	r.counter(name).Add(delta)
}

func (r *Registry) SetGauge(name string, value int64) {
	r.gauge(name).Store(value)
}

func (r *Registry) AddGauge(name string, delta int64) {
	if delta == 0 {
		return
	}
	r.gauge(name).Add(delta)
}

func (r *Registry) ObserveDuration(name string, d time.Duration) {
	r.ObserveMilliseconds(name, float64(d.Milliseconds()))
}

func (r *Registry) ObserveMilliseconds(name string, value float64) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return
	}
	r.histogram(name).observe(value)
}

func (r *Registry) Snapshot() Snapshot {
	r.mu.RLock()
	counterNames := sortedNames(r.counters)
	gaugeNames := sortedNames(r.gauges)
	histogramNames := sortedHistogramNames(r.histograms)
	counters := make(map[string]*atomic.Int64, len(r.counters))
	gauges := make(map[string]*atomic.Int64, len(r.gauges))
	histograms := make(map[string]*histogram, len(r.histograms))
	for _, name := range counterNames {
		counters[name] = r.counters[name]
	}
	for _, name := range gaugeNames {
		gauges[name] = r.gauges[name]
	}
	for _, name := range histogramNames {
		histograms[name] = r.histograms[name]
	}
	r.mu.RUnlock()

	snapshot := Snapshot{
		UptimeSeconds: int64(time.Since(r.startedAt).Seconds()),
		Counters:      make(map[string]int64, len(counters)),
		Gauges:        make(map[string]int64, len(gauges)),
		Histograms:    make(map[string]HistogramSnapshot, len(histograms)),
	}
	for name, counter := range counters {
		snapshot.Counters[name] = counter.Load()
	}
	for name, gauge := range gauges {
		snapshot.Gauges[name] = gauge.Load()
	}
	for name, hist := range histograms {
		snapshot.Histograms[name] = hist.snapshot()
	}
	return snapshot
}

func (r *Registry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(r.Snapshot())
}

func InstrumentHTTP(name string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		Default().IncCounter("http_"+name+"_requests_total", 1)
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next(rw, req)
		Default().ObserveDuration("http_"+name+"_latency_ms", time.Since(start))
		Default().IncCounter("http_"+name+"_status_"+strconv.Itoa(rw.status), 1)
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (r *Registry) counter(name string) *atomic.Int64 {
	r.mu.RLock()
	counter := r.counters[name]
	r.mu.RUnlock()
	if counter != nil {
		return counter
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if counter = r.counters[name]; counter != nil {
		return counter
	}
	counter = &atomic.Int64{}
	r.counters[name] = counter
	return counter
}

func (r *Registry) gauge(name string) *atomic.Int64 {
	r.mu.RLock()
	gauge := r.gauges[name]
	r.mu.RUnlock()
	if gauge != nil {
		return gauge
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if gauge = r.gauges[name]; gauge != nil {
		return gauge
	}
	gauge = &atomic.Int64{}
	r.gauges[name] = gauge
	return gauge
}

func (r *Registry) histogram(name string) *histogram {
	r.mu.RLock()
	h := r.histograms[name]
	r.mu.RUnlock()
	if h != nil {
		return h
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if h = r.histograms[name]; h != nil {
		return h
	}
	h = &histogram{
		bounds: []float64{5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000},
		counts: make([]uint64, 11),
	}
	r.histograms[name] = h
	return h
}

func (h *histogram) observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	idx := len(h.bounds)
	for i, bound := range h.bounds {
		if value <= bound {
			idx = i
			break
		}
	}
	h.counts[idx]++
	h.count++
	h.sum += value
}

func (h *histogram) snapshot() HistogramSnapshot {
	h.mu.Lock()
	defer h.mu.Unlock()

	counts := make([]uint64, len(h.counts))
	copy(counts, h.counts)
	bounds := make([]float64, len(h.bounds))
	copy(bounds, h.bounds)

	avg := 0.0
	if h.count > 0 {
		avg = h.sum / float64(h.count)
	}
	return HistogramSnapshot{
		Count:        h.count,
		Sum:          h.sum,
		Average:      avg,
		Bounds:       bounds,
		BucketCounts: counts,
	}
}

func sortedNames(values map[string]*atomic.Int64) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedHistogramNames(values map[string]*histogram) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
