package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Prometheus bridge: the dashboard in this app talks to Prometheus through
// these handlers, so the browser never needs cross-origin access to the rack.

const defaultPrometheusURL = "http://192.168.1.121:9090"

// PromQL used by the dashboard. All queries are fixed server-side; the client
// can only pick a time window.
const (
	// Host gauges come from max_over_time so a single stale scrape doesn't blank
	// the whole dashboard.
	qCPUPercent = `100 * sum(irate(container_cpu_usage_seconds_total{name!=""}[5m])) / max(max_over_time(machine_cpu_cores[1h]))`
	qCPUCores   = `sum(irate(container_cpu_usage_seconds_total{name!=""}[5m]))`
	qHostCPUs   = `max(max_over_time(machine_cpu_cores[1h]))`
	qMemUsed    = `sum(container_memory_working_set_bytes{name!=""})`
	qMemTotal   = `max(max_over_time(machine_memory_bytes[1h]))`
	qNetRx      = `sum(irate(container_network_receive_bytes_total{name!="",interface!="lo"}[5m]))`
	qNetTx      = `sum(irate(container_network_transmit_bytes_total{name!="",interface!="lo"}[5m]))`
	qContainers = `count(count by (name) (container_last_seen{name!=""}))`

	// Per-container breakdown for the table.
	qByCPU = `sum by (name) (irate(container_cpu_usage_seconds_total{name!=""}[5m]))`
	qByMem = `sum by (name) (container_memory_working_set_bytes{name!=""})`
	qByRx  = `sum by (name) (irate(container_network_receive_bytes_total{name!="",interface!="lo"}[5m]))`
	qByTx  = `sum by (name) (irate(container_network_transmit_bytes_total{name!="",interface!="lo"}[5m]))`
)

type promSample struct {
	labels map[string]string
	value  float64
	at     time.Time
}

type promSeries struct {
	labels map[string]string
	points [][2]float64 // [unix ms, value]
}

type promClient struct {
	base *url.URL
	http *http.Client
}

func newPromClient() *promClient {
	raw := os.Getenv("PROMETHEUS_URL")
	if raw == "" {
		raw = defaultPrometheusURL
	}
	base, err := url.Parse(raw)
	if err != nil {
		base, _ = url.Parse(defaultPrometheusURL)
	}
	return &promClient{
		base: base,
		http: &http.Client{Timeout: 8 * time.Second},
	}
}

type promEnvelope struct {
	Status    string `json:"status"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
	Data      struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  [2]any            `json:"value"`
			Values [][2]any          `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

func (p *promClient) get(ctx context.Context, path string, params url.Values) ([]promSeries, error) {
	endpoint := p.base.ResolveReference(&url.URL{Path: "/api/v1/" + path, RawQuery: params.Encode()})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}

	res, err := p.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var env promEnvelope
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("prometheus: %w", err)
	}
	if env.Status != "success" {
		return nil, fmt.Errorf("prometheus: %s: %s", env.ErrorType, env.Error)
	}

	out := make([]promSeries, 0, len(env.Data.Result))
	for _, r := range env.Data.Result {
		series := promSeries{labels: r.Metric}
		if len(r.Values) > 0 {
			for _, v := range r.Values {
				ts, val, ok := parseSample(v)
				if !ok {
					continue
				}
				series.points = append(series.points, [2]float64{float64(ts.UnixMilli()), val})
			}
		} else if len(r.Value) == 2 {
			if ts, val, ok := parseSample(r.Value); ok {
				series.points = append(series.points, [2]float64{float64(ts.UnixMilli()), val})
				series.labels["__ts"] = strconv.FormatInt(ts.Unix(), 10)
			}
		}
		out = append(out, series)
	}
	return out, nil
}

func parseSample(raw [2]any) (time.Time, float64, bool) {
	f, ok := raw[0].(float64)
	if !ok {
		return time.Time{}, 0, false
	}
	s, ok := raw[1].(string)
	if !ok {
		return time.Time{}, 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return time.Time{}, 0, false
	}
	sec := int64(f)
	nsec := int64((f - float64(sec)) * 1e9)
	return time.Unix(sec, nsec), v, true
}

func (p *promClient) instant(ctx context.Context, expr string) ([]promSample, error) {
	series, err := p.get(ctx, "query", url.Values{"query": {expr}})
	if err != nil {
		return nil, err
	}
	out := make([]promSample, 0, len(series))
	for _, s := range series {
		if len(s.points) == 0 {
			continue
		}
		last := s.points[len(s.points)-1]
		out = append(out, promSample{
			labels: s.labels,
			value:  last[1],
			at:     time.UnixMilli(int64(last[0])),
		})
	}
	return out, nil
}

func (p *promClient) rangeQuery(ctx context.Context, expr string, start, end time.Time, step time.Duration) ([]promSeries, error) {
	return p.get(ctx, "query_range", url.Values{
		"query": {expr},
		"start": {strconv.FormatInt(start.Unix(), 10)},
		"end":   {strconv.FormatInt(end.Unix(), 10)},
		"step":  {strconv.Itoa(int(step.Seconds()))},
	})
}

// scalar returns the first sample value, or 0 when the query has no data.
func (p *promClient) scalar(ctx context.Context, expr string) (float64, error) {
	samples, err := p.instant(ctx, expr)
	if err != nil || len(samples) == 0 {
		return 0, err
	}
	return samples[0].value, nil
}

// fanOut runs several scalar queries concurrently.
func (p *promClient) fanOut(ctx context.Context, exprs ...string) ([]float64, error) {
	values := make([]float64, len(exprs))
	errs := make([]error, len(exprs))
	var wg sync.WaitGroup

	for i, expr := range exprs {
		wg.Add(1)
		go func(i int, expr string) {
			defer wg.Done()
			values[i], errs[i] = p.scalar(ctx, expr)
		}(i, expr)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return values, nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		return
	}
}

func writeJSONError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

type overviewResponse struct {
	CPUPercent float64   `json:"cpuPercent"`
	CPUCores   float64   `json:"cpuCores"`
	HostCPUs   float64   `json:"hostCpus"`
	MemUsed    float64   `json:"memUsed"`
	MemTotal   float64   `json:"memTotal"`
	NetRx      float64   `json:"netRx"`
	NetTx      float64   `json:"netTx"`
	Containers float64   `json:"containers"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (p *promClient) handleOverview(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 9*time.Second)
	defer cancel()

	values, err := p.fanOut(ctx, qCPUPercent, qCPUCores, qHostCPUs, qMemUsed, qMemTotal, qNetRx, qNetTx, qContainers)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, overviewResponse{
		CPUPercent: values[0],
		CPUCores:   values[1],
		HostCPUs:   values[2],
		MemUsed:    values[3],
		MemTotal:   values[4],
		NetRx:      values[5],
		NetTx:      values[6],
		Containers: values[7],
		UpdatedAt:  time.Now().UTC(),
	})
}

var windows = map[string]time.Duration{
	"5m":  5 * time.Minute,
	"15m": 15 * time.Minute,
	"1h":  time.Hour,
	"6h":  6 * time.Hour,
	"24h": 24 * time.Hour,
}

func (p *promClient) handleSeries(w http.ResponseWriter, r *http.Request) {
	windowKey := r.URL.Query().Get("window")
	window, ok := windows[windowKey]
	if !ok {
		windowKey, window = "1h", time.Hour
	}

	// Aim for roughly 60 points across the window, but never below 30s.
	step := window / 60
	if step < 30*time.Second {
		step = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(r.Context(), 9*time.Second)
	defer cancel()

	end := time.Now()
	start := end.Add(-window)

	exprs := []string{qCPUPercent, qMemUsed, qNetRx, qNetTx}
	keys := []string{"cpu", "mem", "rx", "tx"}
	results := make([][]promSeries, len(exprs))
	errs := make([]error, len(exprs))

	var wg sync.WaitGroup
	for i, expr := range exprs {
		wg.Add(1)
		go func(i int, expr string) {
			defer wg.Done()
			results[i], errs[i] = p.rangeQuery(ctx, expr, start, end, step)
		}(i, expr)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, err)
			return
		}
	}

	points := make(map[string][][2]float64, len(exprs))
	for i, key := range keys {
		var series [][2]float64
		if len(results[i]) > 0 {
			series = results[i][0].points
		}
		if series == nil {
			series = [][2]float64{}
		}
		points[key] = series
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"window":    windowKey,
		"step":      int(step.Seconds()),
		"points":    points,
		"updatedAt": time.Now().UTC(),
	})
}

type containerRow struct {
	Name    string  `json:"name"`
	CPU     float64 `json:"cpu"` // percent of the host
	Mem     float64 `json:"mem"` // bytes
	Rx      float64 `json:"rx"`  // bytes/sec
	Tx      float64 `json:"tx"`  // bytes/sec
	Running bool    `json:"running"`
}

func (p *promClient) handleContainers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 9*time.Second)
	defer cancel()

	queries := []string{qByCPU, qByMem, qByRx, qByTx}
	results := make([][]promSample, len(queries))
	errs := make([]error, len(queries))

	var wg sync.WaitGroup
	for i, expr := range queries {
		wg.Add(1)
		go func(i int, expr string) {
			defer wg.Done()
			results[i], errs[i] = p.instant(ctx, expr)
		}(i, expr)
	}
	hostCPUs, err := p.scalar(ctx, qHostCPUs)
	wg.Wait()

	if err == nil {
		for _, e := range errs {
			if e != nil {
				err = e
				break
			}
		}
	}
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err)
		return
	}

	rows := map[string]*containerRow{}
	for i, samples := range results {
		for _, s := range samples {
			name := s.labels["name"]
			if name == "" {
				continue
			}
			row, ok := rows[name]
			if !ok {
				row = &containerRow{Name: name, Running: true}
				rows[name] = row
			}
			switch i {
			case 0:
				if hostCPUs > 0 {
					row.CPU = s.value / hostCPUs * 100
				}
			case 1:
				row.Mem = s.value
			case 2:
				row.Rx = s.value
			case 3:
				row.Tx = s.value
			}
		}
	}

	list := make([]containerRow, 0, len(rows))
	for _, row := range rows {
		list = append(list, *row)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].CPU == list[j].CPU {
			return list[i].Mem > list[j].Mem
		}
		return list[i].CPU > list[j].CPU
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"containers": list,
		"count":      len(list),
		"hostCpus":   hostCPUs,
		"updatedAt":  time.Now().UTC(),
	})
}

func registerMetrics(mux *http.ServeMux) {
	client := newPromClient()
	mux.HandleFunc("GET /api/metrics/overview", client.handleOverview)
	mux.HandleFunc("GET /api/metrics/series", client.handleSeries)
	mux.HandleFunc("GET /api/metrics/containers", client.handleContainers)
}
