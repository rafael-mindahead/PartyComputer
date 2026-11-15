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
	UptimeSeconds float64 `json:"uptime_seconds"`
	Goroutines    int     `json:"goroutines"`
	NumCPU        int     `json:"num_cpu"`
	GoMaxProcs    int     `json:"gomaxprocs"`
	HeapAllocMB   float64 `json:"heap_alloc_mb"`
	SysMemMB      float64 `json:"sys_mem_mb"`
	NumGC         uint32  `json:"num_gc"`
	LastPauseUs   float64 `json:"last_pause_us"`
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

func collectMetrics() MetricsResponse {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	var lastPauseUs float64
	if memStats.NumGC > 0 {
		lastPauseNs := memStats.PauseNs[(memStats.NumGC+255)%256]
		lastPauseUs = math.Round((float64(lastPauseNs)/1000.0)*100) / 100
	}

	return MetricsResponse{
		UptimeSeconds: math.Round(time.Since(startTime).Seconds()),
		Goroutines:    runtime.NumGoroutine(),
		NumCPU:        runtime.NumCPU(),
		GoMaxProcs:    runtime.GOMAXPROCS(0),
		HeapAllocMB:   bytesToMB(memStats.HeapAlloc),
		SysMemMB:      bytesToMB(memStats.Sys),
		NumGC:         memStats.NumGC,
		LastPauseUs:   lastPauseUs,
		GOGC:          int(currentGOGC.Load()),
		MemLimitMB:    getMemLimitMB(),
		GoVersion:     runtime.Version(),
		OSArch:        fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

func getMetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(collectMetrics())
}

// streamMetricsHandler mantém a conexão aberta e envia eventos SSE a cada 500ms
func streamMetricsHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming não suportado", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sendEvent := func() {
		payload, err := json.Marshal(collectMetrics())
		if err != nil {
			return
		}
		// Formato padrão do protocolo SSE: "data: <json>\n\n"
		fmt.Fprintf(w, "data: %s\n\n", payload)
		flusher.Flush()
	}

	// Envia a primeira leitura imediatamente ao conectar
	sendEvent()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			// O navegador fechou a aba ou perdeu conexão: encerra a goroutine
			return
		case <-ticker.C:
			sendEvent()
		}
	}
}

func stressMemoryHandler(w http.ResponseWriter, r *http.Request) {
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
	w.Write([]byte(`{"status":"memory stress started"}`))
}

func stressCPUHandler(w http.ResponseWriter, r *http.Request) {
	workers := runtime.NumCPU()
	deadline := time.Now().Add(3 * time.Second)

	for i := 0; i < workers; i++ {
		go func() {
			x := 1.0001
			for time.Now().Before(deadline) {
				for j := 0; j < 100_000; j++ {
					x = math.Sqrt(x*x + 1.2345)
				}
			}
			_ = x
		}()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"cpu stress started"}`))
}

func forceGCHandler(w http.ResponseWriter, r *http.Request) {
	runtime.GC()
	debug.FreeOSMemory()

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"gc executed"}`))
}

func setGCPercentHandler(w http.ResponseWriter, r *http.Request) {
	var req GCPercentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
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
	mux.HandleFunc("GET /api/metrics", getMetricsHandler)
	mux.HandleFunc("GET /api/stream", streamMetricsHandler)
	mux.HandleFunc("POST /api/stress", stressMemoryHandler)
	mux.HandleFunc("POST /api/stress/cpu", stressCPUHandler)
	mux.HandleFunc("POST /api/gc/run", forceGCHandler)
	mux.HandleFunc("POST /api/gc/percent", setGCPercentHandler)

	fmt.Println("Servidor rodando em http://localhost:8080")
	if err := http.ListenAndServe(":8080", corsMiddleware(mux)); err != nil {
		fmt.Printf("Erro ao iniciar servidor: %v\n", err)
	}
}
