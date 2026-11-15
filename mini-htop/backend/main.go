package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"runtime"
	"runtime/debug"
	"sync/atomic"
	"time"
)

type MetricsResponse struct {
	UpTimeSeconds float64 `json:"uptime_seconds"`
	Goroutines    int     `json:"goroutines"`
	NumCPU        int     `json:"num_cpu"`
	HeapAllocMB   float64 `json:"heap_alloc_mb"`
	SysMemMB      float64 `json:"sys_mem_mb"`
	NumGC         uint32  `json:"num_gc"`
	GOGC          int     `json:"gogc"`
	MemLimitMB    float64 `json:"mem_limit_mb"`
	GoVersion     string  `json:"go_version"`
	OSArch        string  `json:"os_arch"`
}

type GCPercentRequest struct {
	Percent int `json:"percent"`
}

var (
	startTime   = time.Now()
	currentGOGC atomic.Int64
)

func bytesToMB(bytes uint64) float64 {
	mb := float64(bytes) / 1024 / 1024
	return math.Round(mb*100) / 100
}
func getMemLimitMB() float64 {
	limit := debug.SetMemoryLimit(-1)
	if limit == math.MaxInt64 {
		return -1
	}
	return bytesToMB(uint64(limit))
}
func GetMetricsHandler(w http.ResponseWriter, r *http.Request) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	metrics := MetricsResponse{
		UpTimeSeconds: math.Round(float64(time.Since(startTime).Seconds())),
		Goroutines:    runtime.NumGoroutine(),
		NumCPU:        runtime.NumCPU(),
		HeapAllocMB:   bytesToMB(memStats.HeapAlloc),
		SysMemMB:      bytesToMB(memStats.Sys),
		NumGC:         memStats.NumGC,
		GOGC:          int(currentGOGC.Load()),
		MemLimitMB:    getMemLimitMB(),
		GoVersion:     runtime.Version(),
		OSArch:        fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}
func stressHandler(w http.ResponseWriter, r *http.Request) {
	for i := 0; i < 40; i++ {
		go func() {
			buffer := make([]byte, 256*1024)
			for j := range buffer {
				buffer[j] = 1
			}
			time.Sleep(3 * time.Second)
		}()
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status:"stress started""}`))
}
func forceGCHandler(w http.ResponseWriter, r *http.Request) {
	runtime.GC()
	debug.FreeOSMemory()

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status": "gc executed"}`))
}
func setGCPercentHandler(w http.ResponseWriter, r *http.Request){
	var req GCPercentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err !={
		http.Error(w, "JSON invalido", http.StatusBadRequest)
		return
	}
	debug.SetGCPercent(req.Percent)
	currentGOGC.Store(int64(req.Percent))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"gogc": req.Percent})
}
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func main() {
	initialGOGC := debug.SetGCPercent(100)
	debug.SetGCPercent(initialGOGC)
	currentGOGC.Store(int64(initialGOGC))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/metrics", GetMetricsHandler)
	mux.HandleFunc("POST /api/stress", stressHandler)
	mux.HandleFunc("POST /api/gc/run", forceGCHandler)
	mux.HandleFunc("POST /api/gc/percent", setGCPercentHandler)

	fmt.Println("Server is running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", corsMiddleware(mux)); err != nil {
		fmt.Println("error in start the server: %v\n", err)
	}
}
