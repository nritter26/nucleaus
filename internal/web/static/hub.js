/* Mallard Server Hub — navbar, motion and background field */
(() => {
  "use strict";

  const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  const topbar = document.getElementById("topbar");

  /* ---------------- navbar dropdowns ---------------- */

  const items = Array.from(document.querySelectorAll(".nav-item"));

  // Stagger dropdown rows so panels cascade in.
  items.forEach((item) => {
    item.querySelectorAll(".nav-link").forEach((link, i) => {
      link.style.setProperty("--n", i);
    });
  });

  // Dropdowns are anchored to their trigger, which pushes wide panels (Tools)
  // past the right edge. Shift each panel just enough to stay inside the
  // viewport, while keeping the open animation pointing at the trigger.
  const positionPanel = (item) => {
    const panel = item.querySelector(".nav-panel");
    if (!panel) return;

    if (window.matchMedia("(max-width: 1024px)").matches) {
      panel.style.removeProperty("--nav-panel-left");
      panel.style.removeProperty("--nav-panel-origin");
      return;
    }

    const rect = item.getBoundingClientRect();
    const width = panel.offsetWidth;
    const viewport = document.documentElement.clientWidth;
    const margin = 16;
    const left = Math.max(margin, Math.min(rect.left, viewport - margin - width));

    panel.style.setProperty("--nav-panel-left", `${Math.round(left - rect.left)}px`);
    panel.style.setProperty(
      "--nav-panel-origin",
      `${Math.round(rect.left + rect.width / 2 - left)}px`
    );
  };

  // Position closed panels too: their absolute box still counts towards the
  // document's scroll width, which made every page horizontally scrollable.
  items.forEach(positionPanel);

  const closeAll = (except) => {
    items.forEach((item) => {
      if (item === except) return;
      item.dataset.open = "false";
      const trigger = item.querySelector(".nav-trigger");
      if (trigger) trigger.setAttribute("aria-expanded", "false");
    });
  };

  items.forEach((item) => {
    const trigger = item.querySelector(".nav-trigger");
    if (!trigger) return;
    item.dataset.open = "false";

    // Plain links (nav-trigger--link) navigate normally; only fake the
    // dropdown open/close state for actual dropdown triggers.
    if (trigger.classList.contains("nav-trigger--link")) return;

    trigger.addEventListener("click", (event) => {
      event.preventDefault();
      positionPanel(item);
      const open = item.dataset.open !== "true";
      closeAll(item);
      item.dataset.open = String(open);
      trigger.setAttribute("aria-expanded", String(open));
    });

    item.addEventListener("pointerenter", () => positionPanel(item));
    item.addEventListener("focusin", () => positionPanel(item));

    item.addEventListener("keydown", (event) => {
      if (event.key === "Escape") {
        item.dataset.open = "false";
        trigger.setAttribute("aria-expanded", "false");
        trigger.focus();
      }
    });
  });

  document.addEventListener("click", (event) => {
    if (!event.target.closest(".nav-item")) closeAll(null);
  });

  // Trigger positions move on resize and after webfonts load.
  let panelTimer = 0;
  const reposition = () => {
    clearTimeout(panelTimer);
    panelTimer = window.setTimeout(() => items.forEach(positionPanel), 120);
  };
  window.addEventListener("resize", reposition);
  window.addEventListener("load", reposition);
  if (document.fonts && document.fonts.ready) {
    document.fonts.ready.then(reposition);
  }

  /* ---------------- mobile menu ---------------- */

  const toggle = document.querySelector(".nav-toggle");
  if (toggle && topbar) {
    toggle.addEventListener("click", () => {
      const open = !topbar.classList.contains("nav-open");
      topbar.classList.toggle("nav-open", open);
      toggle.setAttribute("aria-expanded", String(open));
      if (!open) closeAll(null);
    });
  }

  /* ---------------- scroll reveal ---------------- */

  const revealables = document.querySelectorAll(".reveal");
  if (reduced || !("IntersectionObserver" in window)) {
    revealables.forEach((el) => el.classList.add("is-in"));
  } else {
    const io = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (!entry.isIntersecting) return;
          entry.target.classList.add("is-in");
          io.unobserve(entry.target);
        });
      },
      { rootMargin: "0px 0px -8% 0px", threshold: 0.12 }
    );
    revealables.forEach((el) => io.observe(el));
  }

  /* ---------------- card spotlight ---------------- */

  if (!reduced && window.matchMedia("(hover: hover)").matches) {
    document.querySelectorAll(".card").forEach((card) => {
      card.addEventListener("pointermove", (event) => {
        const rect = card.getBoundingClientRect();
        const x = ((event.clientX - rect.left) / rect.width) * 100;
        const y = ((event.clientY - rect.top) / rect.height) * 100;
        card.style.setProperty("--mx", `${x}%`);
        card.style.setProperty("--my", `${y}%`);
      });
    });
  }

  /* ---------------- rail active state ---------------- */

  const railLinks = Array.from(document.querySelectorAll(".rail-link"));
  if (railLinks.length && "IntersectionObserver" in window) {
    const sections = railLinks
      .map((a) => document.querySelector(a.getAttribute("href")))
      .filter(Boolean);

    const nav = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (!entry.isIntersecting) return;
          railLinks.forEach((a) =>
            a.classList.toggle("is-active", a.getAttribute("href") === `#${entry.target.id}`)
          );
        });
      },
      { rootMargin: "-45% 0px -45% 0px" }
    );
    sections.forEach((section) => nav.observe(section));
  }

  /* ---------------- starfield ---------------- */

  const canvas = document.getElementById("stars");
  if (!canvas || reduced) return;

  const ctx = canvas.getContext("2d");
  if (!ctx) return;

  let width = 0;
  let height = 0;
  let stars = [];
  let raf = 0;

  const config = {
    count: 0,
    speed: 0.12,
    colors: ["175, 5, 202", "4, 134, 192", "34, 211, 238", "255, 255, 255"],
  };

  // Pre-render one glow sprite per color so the frame loop is a cheap
  // drawImage per star instead of two gradient arcs.
  const sprite = (color) => {
    const s = document.createElement("canvas");
    const size = 32;
    s.width = s.height = size;
    const c = s.getContext("2d");
    const [r, g, b] = color.split(", ");
    const grad = c.createRadialGradient(size / 2, size / 2, 0, size / 2, size / 2, size / 2);
    grad.addColorStop(0, `rgba(${r}, ${g}, ${b}, 1)`);
    grad.addColorStop(0.28, `rgba(${r}, ${g}, ${b}, 0.5)`);
    grad.addColorStop(1, `rgba(${r}, ${g}, ${b}, 0)`);
    c.fillStyle = grad;
    c.fillRect(0, 0, size, size);
    return s;
  };

  let sprites = [];

  const resize = () => {
    const dpr = Math.min(window.devicePixelRatio || 1, 1.25);
    width = canvas.clientWidth;
    height = canvas.clientHeight;
    canvas.width = Math.floor(width * dpr);
    canvas.height = Math.floor(height * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);

    if (!sprites.length) {
      sprites = config.colors.map(sprite);
    }

    config.count = Math.round(Math.min(90, Math.max(30, (width * height) / 24000)));
    stars = Array.from({ length: config.count }, () => ({
      x: Math.random() * width,
      y: Math.random() * height,
      s: Math.random() * 5 + 2,
      vy: Math.random() * 0.35 + 0.06,
      drift: (Math.random() - 0.5) * 0.12,
      alpha: Math.random() * 0.5 + 0.12,
      twinkle: Math.random() * Math.PI * 2,
      img: sprites[Math.floor(Math.random() * sprites.length)],
    }));
  };

  const draw = () => {
    ctx.clearRect(0, 0, width, height);

    for (const star of stars) {
      star.y -= star.vy;
      star.x += star.drift;
      star.twinkle += 0.02;

      if (star.y < -4) {
        star.y = height + 4;
        star.x = Math.random() * width;
      }
      if (star.x < -4) star.x = width + 4;
      if (star.x > width + 4) star.x = -4;

      const alpha = star.alpha * (0.65 + 0.35 * Math.sin(star.twinkle));
      ctx.globalAlpha = alpha > 1 ? 1 : alpha;
      ctx.drawImage(star.img, star.x - star.s / 2, star.y - star.s / 2, star.s, star.s);
    }
    ctx.globalAlpha = 1;

    raf = requestAnimationFrame(draw);
  };

  // Cap the field at ~30fps: it's ambient motion, not gameplay.
  const WAIT = 1000 / 30;
  let last = 0;
  const tick = (t) => {
    if (t - last >= WAIT) {
      last = t;
      draw();
    } else {
      raf = requestAnimationFrame(tick);
    }
  };

  resize();
  tick(0);

  let resizeTimer = 0;
  window.addEventListener("resize", () => {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(resize, 160);
  });

  document.addEventListener("visibilitychange", () => {
    if (document.hidden) {
      cancelAnimationFrame(raf);
    } else {
      last = 0;
      raf = requestAnimationFrame(tick);
    }
  });
})();
