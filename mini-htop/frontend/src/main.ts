import './style.css';

// Espelha exatamente a struct MetricsResponse do Go
interface SystemMetrics {
  uptime_seconds: number;
  goroutines: number;
  num_cpu: number;
  heap_alloc_mb: number;
  sys_mem_mb: number;
  num_gc: number;
  go_version: string;
  os_arch: string;
}

const API_BASE = 'http://localhost:8080/api';
const MAX_HISTORY = 30;
const heapHistory: number[] = [];

// Referências tipadas do DOM
const systemInfoEl = document.getElementById('system-info') as HTMLParagraphElement;
const statusBadgeEl = document.getElementById('status-badge') as HTMLSpanElement;
const stressBtn = document.getElementById('stress-btn') as HTMLButtonElement;
const heapValueEl = document.getElementById('heap-value') as HTMLElement;
const heapBarEl = document.getElementById('heap-bar') as HTMLDivElement;
const sysValueEl = document.getElementById('sys-value') as HTMLElement;
const goroutinesValueEl = document.getElementById('goroutines-value') as HTMLElement;
const goroutinesBarEl = document.getElementById('goroutines-bar') as HTMLDivElement;
const cpuValueEl = document.getElementById('cpu-value') as HTMLElement;
const uptimeValueEl = document.getElementById('uptime-value') as HTMLElement;
const gcValueEl = document.getElementById('gc-value') as HTMLElement;
const peakValueEl = document.getElementById('peak-value') as HTMLSpanElement;
const sparklineEl = document.getElementById('sparkline') as HTMLDivElement;

function formatUptime(totalSeconds: number): string {
  const mins = Math.floor(totalSeconds / 60);
  const secs = totalSeconds % 60;
  return mins > 0 ? `${mins}m ${secs}s` : `${secs}s`;
}

function renderSparkline(): void {
  const peak = Math.max(...heapHistory, 1);
  peakValueEl.textContent = `Pico: ${peak.toFixed(2)} MB`;

  sparklineEl.innerHTML = heapHistory
    .map((val) => {
      const heightPct = Math.min((val / peak) * 100, 100);
      return `<div class="spark-bar" style="height: ${heightPct}%" title="${val} MB"></div>`;
    })
    .join('');
}

function updateUI(data: SystemMetrics): void {
  statusBadgeEl.textContent = 'Online';
  statusBadgeEl.className = 'badge online';
  systemInfoEl.textContent = `${data.go_version} • ${data.os_arch}`;

  // Memória (escala visual relativa ao reservado pelo SO)
  heapValueEl.textContent = `${data.heap_alloc_mb.toFixed(2)} MB`;
  sysValueEl.textContent = `Reservado pelo SO: ${data.sys_mem_mb.toFixed(2)} MB`;
  const memPct = Math.min((data.heap_alloc_mb / Math.max(data.sys_mem_mb, 1)) * 100, 100);
  heapBarEl.style.width = `${memPct}%`;

  // Goroutines (escala visual até 60 goroutines = 100%)
  goroutinesValueEl.textContent = String(data.goroutines);
  cpuValueEl.textContent = `Núcleos lógicos: ${data.num_cpu}`;
  const goroutinePct = Math.min((data.goroutines / 60) * 100, 100);
  goroutinesBarEl.style.width = `${goroutinePct}%`;

  // Uptime & GC
  uptimeValueEl.textContent = formatUptime(data.uptime_seconds);
  gcValueEl.textContent = `Ciclos de GC: ${data.num_gc}`;

  // Histórico
  heapHistory.push(data.heap_alloc_mb);
  if (heapHistory.length > MAX_HISTORY) {
    heapHistory.shift();
  }
  renderSparkline();
}

async function fetchMetrics(): Promise<void> {
  try {
    const response = await fetch(`${API_BASE}/metrics`);
    if (!response.ok) throw new Error('Falha na resposta');

    const metrics: SystemMetrics = await response.json();
    updateUI(metrics);
  } catch {
    statusBadgeEl.textContent = 'Offline';
    statusBadgeEl.className = 'badge offline';
  }
}

stressBtn.addEventListener('click', async () => {
  stressBtn.disabled = true;
  stressBtn.textContent = 'Alocando carga...';

  try {
    await fetch(`${API_BASE}/stress`, { method: 'POST' });
  } finally {
    setTimeout(() => {
      stressBtn.disabled = false;
      stressBtn.textContent = 'Simular Carga (3s)';
    }, 3000);
  }
});

// Primeira leitura imediata + polling a cada 1 segundo
fetchMetrics();
setInterval(fetchMetrics, 1000);
