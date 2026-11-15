import './style.css';

interface SystemMetrics {
  uptime_seconds: number;
  goroutines: number;
  num_cpu: number;
  gomaxprocs: number;
  heap_alloc_mb: number;
  sys_mem_mb: number;
  num_gc: number;
  last_pause_us: number;
  gogc: number;
  mem_limit_mb: number;
  go_version: string;
  os_arch: string;
}

const API_BASE = 'http://localhost:8080/api';
const MAX_HISTORY = 30;
const heapHistory: number[] = [];

const systemInfoEl = document.getElementById('system-info') as HTMLParagraphElement;
const statusBadgeEl = document.getElementById('status-badge') as HTMLSpanElement;
const stressBtn = document.getElementById('stress-btn') as HTMLButtonElement;
const cpuBtn = document.getElementById('cpu-btn') as HTMLButtonElement;
const forceGcBtn = document.getElementById('force-gc-btn') as HTMLButtonElement;
const gogcButtons = document.querySelectorAll<HTMLButtonElement>('#gogc-buttons .pill-btn');

const heapValueEl = document.getElementById('heap-value') as HTMLElement;
const heapBarEl = document.getElementById('heap-bar') as HTMLDivElement;
const sysValueEl = document.getElementById('sys-value') as HTMLElement;
const goroutinesValueEl = document.getElementById('goroutines-value') as HTMLElement;
const goroutinesBarEl = document.getElementById('goroutines-bar') as HTMLDivElement;
const cpuValueEl = document.getElementById('cpu-value') as HTMLElement;
const uptimeValueEl = document.getElementById('uptime-value') as HTMLElement;
const gcValueEl = document.getElementById('gc-value') as HTMLElement;
const memLimitValueEl = document.getElementById('memlimit-value') as HTMLElement;
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
  statusBadgeEl.textContent = 'Live (SSE)';
  statusBadgeEl.className = 'badge online';
  systemInfoEl.textContent = `${data.go_version} • ${data.os_arch}`;

  heapValueEl.textContent = `${data.heap_alloc_mb.toFixed(2)} MB`;
  sysValueEl.textContent = `Reservado pelo SO: ${data.sys_mem_mb.toFixed(2)} MB`;
  const memPct = Math.min((data.heap_alloc_mb / Math.max(data.sys_mem_mb, 1)) * 100, 100);
  heapBarEl.style.width = `${memPct}%`;

  goroutinesValueEl.textContent = String(data.goroutines);
  cpuValueEl.textContent = `Núcleos: ${data.num_cpu} • GOMAXPROCS: ${data.gomaxprocs}`;
  const goroutinePct = Math.min((data.goroutines / 60) * 100, 100);
  goroutinesBarEl.style.width = `${goroutinePct}%`;

  uptimeValueEl.textContent = formatUptime(data.uptime_seconds);
  gcValueEl.textContent = `Ciclos de GC: ${data.num_gc} (Pausa: ${data.last_pause_us} µs)`;
  memLimitValueEl.textContent =
    data.mem_limit_mb < 0 ? 'GOMEMLIMIT: Ilimitado' : `GOMEMLIMIT: ${data.mem_limit_mb.toFixed(2)} MB`;

  gogcButtons.forEach((btn) => {
    const val = Number(btn.dataset.gogc);
    btn.classList.toggle('active', val === data.gogc);
  });

  heapHistory.push(data.heap_alloc_mb);
  if (heapHistory.length > MAX_HISTORY) {
    heapHistory.shift();
  }
  renderSparkline();
}

function connectStream(): void {
  const eventSource = new EventSource(`${API_BASE}/stream`);

  eventSource.onmessage = (event: MessageEvent<string>) => {
    const metrics: SystemMetrics = JSON.parse(event.data);
    updateUI(metrics);
  };

  eventSource.onerror = () => {
    statusBadgeEl.textContent = 'Reconectando...';
    statusBadgeEl.className = 'badge offline';
  };
}

stressBtn.addEventListener('click', async () => {
  stressBtn.disabled = true;
  stressBtn.textContent = 'Alocando memória...';

  try {
    await fetch(`${API_BASE}/stress`, { method: 'POST' });
  } finally {
    setTimeout(() => {
      stressBtn.disabled = false;
      stressBtn.textContent = 'Simular Memória (3s)';
    }, 3000);
  }
});

cpuBtn.addEventListener('click', async () => {
  cpuBtn.disabled = true;
  cpuBtn.textContent = 'CPU a 100%...';

  try {
    await fetch(`${API_BASE}/stress/cpu`, { method: 'POST' });
  } finally {
    setTimeout(() => {
      cpuBtn.disabled = false;
      cpuBtn.textContent = 'Simular CPU (3s)';
    }, 3000);
  }
});

forceGcBtn.addEventListener('click', async () => {
  forceGcBtn.disabled = true;
  try {
    await fetch(`${API_BASE}/gc/run`, { method: 'POST' });
  } finally {
    forceGcBtn.disabled = false;
  }
});

gogcButtons.forEach((btn) => {
  btn.addEventListener('click', async () => {
    const percent = Number(btn.dataset.gogc);
    await fetch(`${API_BASE}/gc/percent`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ percent }),
    });
  });
});

connectStream();
