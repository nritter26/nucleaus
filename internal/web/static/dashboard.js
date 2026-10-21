/* Mallard Server Hub — live telemetry dashboard driven by /api/metrics */
(() => {
  "use strict";

  const root = document.getElementById("telemetry");
  if (!root) return;

  const source = root.dataset.source || "/api/metrics";
  const statusEl = root.querySelector('[data-field="status"]');
  const statusWrap = document.getElementById("telemetry-status");
  const rowsEl = document.getElementById("container-rows");
  const countEl = root.querySelector('[data-field="containerCount"]');

  const colors = { cpu: "#af05ca", mem: "#0486c0", rx: "#22d3ee", tx: "#ff5bd8" };
  const fields = {
    cpu: { points: "cpu", scale: (v) => v, unit: "%" },
    mem: { points: "mem", scale: (v) => v / 1024 ** 3, unit: "GiB" },
    rx: { points: "rx", scale: (v) => v, unit: "B/s" },
    tx: { points: "tx", scale: (v) => v, unit: "B/s" },
  };

  let windowKey = "1h";
  let lastUpdate = 0;
  let failed = false;
  let seriesTimer = 0;

  /* ---------------- formatting ---------------- */

  const fmtBytes = (n) => {
    if (!Number.isFinite(n) || n <= 0) return "0 B";
    const units = ["B", "KiB", "MiB", "GiB", "TiB"];
    const i = Math.min(units.length - 1, Math.floor(Math.log(n) / Math.log(1024)));
    const value = n / 1024 ** i;
    return `${value >= 100 || i === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[i]}`;
  };

  const fmtRate = (n) => `${fmtBytes(n)}/s`;
  const fmtGib = (n) => (n / 1024 ** 3).toFixed(1);
  const fmtPct = (n) => (Number.isFinite(n) ? n.toFixed(1) : "0.0");

  const fmtClock = (ms) => {
    const d = new Date(ms);
    return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
  };

  /* ---------------- fetch ---------------- */

  const getJSON = async (path) => {
    const res = await fetch(`${source}${path}`, {
      headers: { Accept: "application/json" },
      cache: "no-store",
    });
    const body = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(body.error || `HTTP ${res.status}`);
    return body;
  };

  const setStatus = (text, isError) => {
    if (statusEl) statusEl.textContent = text;
    if (statusWrap) statusWrap.classList.toggle("is-error", Boolean(isError));
  };

  const markUpdated = () => {
    lastUpdate = Date.now();
    failed = false;
    root.querySelectorAll(".chart-card").forEach((c) => c.classList.remove("is-stale"));
    setStatus(`Live · updated ${new Date().toLocaleTimeString()}`, false);
  };

  const markFailed = (err) => {
    failed = true;
    root.querySelectorAll(".chart-card").forEach((c) => c.classList.add("is-stale"));
    setStatus(`Bridge offline — ${err.message}`, true);
  };

  /* ---------------- charts ---------------- */

  const niceMax = (max) => {
    if (max <= 0) return 1;
    const exp = Math.floor(Math.log10(max));
    const base = 10 ** exp;
    const scaled = max / base;
    const step = scaled <= 1 ? 1 : scaled <= 2 ? 2 : scaled <= 5 ? 5 : 10;
    return step * base;
  };

  const drawChart = (card, rawPoints, key) => {
    const canvas = card.querySelector("canvas");
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const width = canvas.clientWidth || 320;
    const height = canvas.clientHeight || 180;
    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    canvas.width = Math.round(width * dpr);
    canvas.height = Math.round(height * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, width, height);

    const spec = fields[key];
    const points = rawPoints
      .map(([ts, value]) => [ts, spec.scale(value)])
      .filter(([, v]) => Number.isFinite(v));

    card.classList.toggle("has-data", points.length > 0);
    if (!points.length) return;

    const pad = { top: 12, right: 10, bottom: 18, left: 8 };
    const plotW = width - pad.left - pad.right;
    const plotH = height - pad.top - pad.bottom;

    const maxY = niceMax(Math.max(...points.map((p) => p[1])));
    const minX = points[0][0];
    const maxX = points[points.length - 1][0] || minX + 1000;
    const spanX = Math.max(1, maxX - minX);

    const x = (ts) => pad.left + ((ts - minX) / spanX) * plotW;
    const y = (v) => pad.top + plotH - (v / maxY) * plotH;

    // grid
    ctx.strokeStyle = "rgba(255, 255, 255, 0.06)";
    ctx.lineWidth = 1;
    for (let i = 0; i <= 3; i += 1) {
      const gy = pad.top + (plotH / 3) * i;
      ctx.beginPath();
      ctx.moveTo(pad.left, gy);
      ctx.lineTo(width - pad.right, gy);
      ctx.stroke();
    }

    // area
    const gradient = ctx.createLinearGradient(0, pad.top, 0, pad.top + plotH);
    gradient.addColorStop(0, `${colors[key]}66`);
    gradient.addColorStop(1, `${colors[key]}00`);
    ctx.beginPath();
    ctx.moveTo(x(points[0][0]), y(points[0][1]));
    points.forEach(([ts, v]) => ctx.lineTo(x(ts), y(v)));
    ctx.lineTo(x(points[points.length - 1][0]), pad.top + plotH);
    ctx.lineTo(x(points[0][0]), pad.top + plotH);
    ctx.closePath();
    ctx.fillStyle = gradient;
    ctx.fill();

    // line
    ctx.beginPath();
    points.forEach(([ts, v], i) => {
      const px = x(ts);
      const py = y(v);
      if (i === 0) ctx.moveTo(px, py);
      else ctx.lineTo(px, py);
    });
    ctx.strokeStyle = colors[key];
    ctx.lineWidth = 2;
    ctx.lineJoin = "round";
    ctx.lineCap = "round";
    ctx.shadowColor = colors[key];
    ctx.shadowBlur = 10;
    ctx.stroke();
    ctx.shadowBlur = 0;

    // last point
    const last = points[points.length - 1];
    ctx.beginPath();
    ctx.fillStyle = "#fff";
    ctx.arc(x(last[0]), y(last[1]), 3, 0, Math.PI * 2);
    ctx.fill();

    // labels
    ctx.fillStyle = "rgba(162, 148, 189, 0.85)";
    ctx.font = '10px "JetBrains Mono", ui-monospace, monospace';
    ctx.textAlign = "left";
    ctx.fillText(`${spec.unit === "B/s" ? fmtBytes(maxY) + "/s" : `${maxY.toFixed(maxY < 10 ? 1 : 0)} ${spec.unit}`}`, pad.left, pad.top - 2);
    ctx.textAlign = "left";
    ctx.fillText(fmtClock(minX), pad.left, height - 3);
    ctx.textAlign = "right";
    ctx.fillText(fmtClock(maxX), width - pad.right, height - 3);
    ctx.textAlign = "right";
    ctx.fillStyle = "rgba(236, 231, 247, 0.9)";
    ctx.font = '600 11px "JetBrains Mono", ui-monospace, monospace';
    const label = spec.unit === "B/s" ? fmtRate(last[1]) : spec.unit === "GiB" ? `${last[1].toFixed(2)} GiB` : `${last[1].toFixed(1)}%`;
    ctx.fillText(label, width - pad.right, pad.top - 2);
  };

  /* ---------------- sections ---------------- */

  const setField = (name, value) => {
    const el = root.querySelector(`[data-field="${name}"]`);
    if (el) el.textContent = value;
  };

  const setBar = (name, pct) => {
    const el = root.querySelector(`[data-field="${name}"]`);
    if (el) el.style.width = `${Math.max(0, Math.min(100, pct || 0))}%`;
  };

  const renderOverview = (data) => {
    const hasHost = data.hostCpus > 0;
    const memPct = data.memTotal > 0 ? (data.memUsed / data.memTotal) * 100 : 0;
    setField("cpuPercent", hasHost ? fmtPct(data.cpuPercent) : "—");
    setField("cpuSub", hasHost ? `${data.cpuCores.toFixed(2)} of ${Math.round(data.hostCpus)} cores` : "host metrics unavailable");
    setBar("cpuBar", data.cpuPercent);

    setField("memPercent", data.memTotal > 0 ? fmtPct(memPct) : "—");
    setField("memSub", data.memTotal > 0 ? `${fmtGib(data.memUsed)} / ${fmtGib(data.memTotal)} GiB` : `${fmtGib(data.memUsed)} GiB used`);
    setBar("memBar", memPct);

    setField("netRx", fmtRate(data.netRx));
    setField("netRxSub", `${fmtBytes(data.netRx * 60)} / min`);
    setField("netTx", fmtRate(data.netTx));
    setField("netTxSub", `${fmtBytes(data.netTx * 60)} / min`);
    setField("containers", String(Math.round(data.containers)));
  };

  const renderSeries = (data) => {
    root.querySelectorAll(".chart-card").forEach((card) => {
      const key = card.dataset.chart;
      const spec = fields[key];
      if (!spec) return;
      const points = (data.points && data.points[spec.points]) || [];
      drawChart(card, points, key);
    });
  };

  const renderContainers = (data) => {
    if (!rowsEl) return;
    const rows = (data.containers || []).slice(0, 24);
    if (countEl) countEl.textContent = `${data.count || rows.length} tracked`;

    if (!rows.length) {
      rowsEl.innerHTML = '<tr><td colspan="5" class="table-empty">no containers reported</td></tr>';
      return;
    }

    rowsEl.innerHTML = rows
      .map((row) => {
        const cpu = Math.max(0, Math.min(100, row.cpu || 0));
        return `<tr>
          <td>${escapeHTML(row.name)}</td>
          <td><span class="cpu-meter"><i style="width:${cpu.toFixed(1)}%"></i></span>${cpu.toFixed(2)}%</td>
          <td>${fmtGib(row.mem)} GiB</td>
          <td>${fmtRate(row.rx)}</td>
          <td>${fmtRate(row.tx)}</td>
        </tr>`;
      })
      .join("");
  };

  const escapeHTML = (value) =>
    String(value).replace(/[&<>"']/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[char]));

  /* ---------------- polling ---------------- */

  const loadSeries = async () => {
    const data = await getJSON(`/series?window=${encodeURIComponent(windowKey)}`);
    renderSeries(data);
  };

  const loadLive = async () => {
    const [overview, containers] = await Promise.all([
      getJSON("/overview"),
      getJSON("/containers"),
    ]);
    renderOverview(overview);
    renderContainers(containers);
    markUpdated();
  };

  const refresh = async () => {
    try {
      await loadLive();
    } catch (err) {
      markFailed(err);
    }
  };

  const refreshSeries = async () => {
    try {
      await loadSeries();
    } catch (err) {
      markFailed(err);
    }
  };

  const start = () => {
    refresh();
    refreshSeries();
    clearInterval(seriesTimer);
    seriesTimer = setInterval(() => {
      if (!document.hidden) refreshSeries();
    }, 45000);
  };

  document.querySelectorAll(".window-switch button").forEach((button) => {
    button.addEventListener("click", () => {
      if (button.dataset.window === windowKey) return;
      windowKey = button.dataset.window;
      document.querySelectorAll(".window-switch button").forEach((b) => b.classList.toggle("is-active", b === button));
      refreshSeries();
    });
  });

  setInterval(() => {
    if (!document.hidden) refresh();
  }, 10000);

  window.addEventListener("resize", () => {
    if (!lastUpdate || failed) return;
    loadSeries().catch(() => {});
  });

  document.addEventListener("visibilitychange", () => {
    if (!document.hidden) refresh();
  });

  start();
})();
