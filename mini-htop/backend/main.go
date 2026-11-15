package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"runtime"
	"time"
)

type MetricsResponse struct {
	UpTimeSeconds float64 `json:"uptime_seconds"`
	Goroutines    int     `json:"goroutines"`
	NumCPU        int     `json:"num_cpu"`
	HeapAllocMB   float64 `json:"heap_alloc_mb"`
	SysMemMB      float64 `json:"sys_mem_mb"`
	NumGC         uint32  `json:"num_gc"`
	GoVersion     string  `json:"go_version"`
	OSArch        string  `json:"os_arch"`
}

var startTime = time.Now()

func bytesToMB(bytes uint64) float64 {
	mb := float64(bytes) / 1024 / 1024
	return math.Round(mb*100) / 100
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
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/metrics", GetMetricsHandler)
	mux.HandleFunc("POST /api/stress", stressHandler)

	fmt.Println("Server is running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", corsMiddleware(mux)); err != nil {
		fmt.Println("error in start the server: %v\n", err)
	}
}
