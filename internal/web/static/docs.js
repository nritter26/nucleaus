/* Mallard Server Hub — curriculum & snippet docs: search, tabs, copy */
(() => {
  "use strict";

  const search = document.getElementById("doc-search");
  const tabs = document.getElementById("doc-tabs");
  const groups = Array.from(document.querySelectorAll(".doc-group"));
  const sections = Array.from(document.querySelectorAll(".doc-sec"));
  const countEl = document.getElementById("doc-count");
  const noneEl = document.getElementById("doc-none");

  if (!sections.length) return;

  let activeGroup = "all";

  const apply = () => {
    const query = search ? search.value.trim().toLowerCase() : "";
    let visible = 0;

    sections.forEach((sec) => {
      const group = sec.closest(".doc-group");
      const groupOk = activeGroup === "all" || (group && group.dataset.doc === activeGroup);
      const hay = (sec.dataset.search || "").toLowerCase();
      const show = groupOk && (!query || hay.includes(query));
      sec.hidden = !show;
      sec.open = show && (!!query || sec.dataset.first === "1");
      if (show) visible += 1;
    });

    groups.forEach((group) => {
      group.hidden =
        group.querySelectorAll(".doc-sec:not([hidden])").length === 0;
    });

    if (countEl) countEl.textContent = `${visible} section${visible === 1 ? "" : "s"}`;
    if (noneEl) noneEl.hidden = visible !== 0;
  };

  if (search) {
    search.addEventListener("input", apply);

    document.addEventListener("keydown", (event) => {
      if (event.key === "/" && document.activeElement !== search) {
        event.preventDefault();
        search.focus();
        search.select();
      }
    });
  }

  // Color the code blocks once the tokenizer has loaded (defer order).
  window.hubHighlight && window.hubHighlight();

  if (tabs) {
    tabs.addEventListener("click", (event) => {
      const button = event.target.closest("button[data-doc]");
      if (!button) return;

      activeGroup = button.dataset.doc;
      tabs.querySelectorAll("button").forEach((tab) =>
        tab.classList.toggle("is-active", tab === button)
      );
      apply();

      const target = document.querySelector(`.doc-group[data-doc="${activeGroup}"]`);
      if (activeGroup === "all" || !target) return;
      target.scrollIntoView({ behavior: "smooth", block: "start" });
    });
  }

  // Deep links to a section anchor open it, even when it was loaded closed.
  const openAnchor = () => {
    if (!location.hash) return;
    const target = document.getElementById(location.hash.slice(1));
    if (target && target.classList.contains("doc-sec")) {
      target.open = true;
      target.classList.add("is-in");
      target.scrollIntoView({ behavior: "smooth", block: "start" });
    }
  };
  window.addEventListener("hashchange", openAnchor);
  if (location.hash) openAnchor();

  // The hub runs over plain HTTP on a LAN address, where navigator.clipboard is
  // unavailable, so fall back to a hidden textarea + execCommand.
  const copyText = async (text) => {
    if (navigator.clipboard && window.isSecureContext) {
      try {
        await navigator.clipboard.writeText(text);
        return true;
      } catch (err) {
        /* fall through to the legacy path */
      }
    }

    const area = document.createElement("textarea");
    area.value = text;
    area.setAttribute("readonly", "");
    area.style.cssText = "position:fixed;top:-1000px;opacity:0";
    document.body.appendChild(area);
    area.select();

    let ok = false;
    try {
      ok = document.execCommand("copy");
    } catch (err) {
      ok = false;
    }
    area.remove();
    return ok;
  };

  document.querySelectorAll("[data-copy]").forEach((button) => {
    button.addEventListener("click", async () => {
      const code = button.closest(".doc-code")?.querySelector("code");
      if (!code) return;

      // Copy the original text, not the highlighted HTML.
      const ok = await copyText(code.textContent);
      button.textContent = ok ? "copied" : "failed";
      button.classList.toggle("is-done", ok);

      window.setTimeout(() => {
        button.textContent = "copy";
        button.classList.remove("is-done");
      }, 1400);
    });
  });

  // Curriculum progress: lessons are marked complete in localStorage so a
  // learner can leave and resume without an account. Anything stored by an
  // older build (or corrupted JSON) falls back to an empty set.
  const progress = document.querySelector(".doc-progress");
  if (progress) {
    const key = `mallard:progress:${progress.dataset.progress}`;
    const lessons = sections.filter((sec) => sec.dataset.lesson === "1");
    const doneEl = document.getElementById("doc-progress-done");
    const barEl = document.getElementById("doc-progress-bar");
    const trackEl = document.getElementById("doc-progress-track");
    const nextEl = document.getElementById("doc-progress-next");
    const resetEl = document.getElementById("doc-progress-reset");

    const read = () => {
      try {
        const value = JSON.parse(localStorage.getItem(key) || "[]");
        return new Set(Array.isArray(value) ? value : []);
      } catch (err) {
        return new Set();
      }
    };

    let done = read();

    const write = () => {
      try {
        localStorage.setItem(key, JSON.stringify([...done]));
      } catch (err) {
        /* storage disabled: progress still works for this page view */
      }
    };

    const syncProgress = () => {
      let count = 0;
      lessons.forEach((sec) => {
        const complete = done.has(sec.id);
        sec.classList.toggle("is-done", complete);

        const button = sec.querySelector(".doc-done");
        if (button) {
          button.setAttribute("aria-pressed", complete ? "true" : "false");
          const label = button.querySelector(".doc-done-label");
          if (label) {
            label.textContent = complete ? "Lesson complete" : "Mark lesson complete";
          }
        }

        if (complete) count += 1;
      });

      if (doneEl) doneEl.textContent = String(count);
      if (barEl) {
        barEl.style.width = `${lessons.length ? Math.round((count / lessons.length) * 100) : 0}%`;
      }
      if (trackEl) trackEl.setAttribute("aria-valuenow", String(count));

      const next = lessons.find((sec) => !done.has(sec.id));
      if (nextEl) {
        nextEl.hidden = !next;
        if (next) {
          const name = next.querySelector(".doc-sec-name")?.textContent?.trim() || "the next lesson";
          nextEl.href = `#${next.id}`;
          nextEl.textContent = count === 0 ? "Start the course →" : "Continue →";
          nextEl.title = name;
        }
      }
    };

    document.addEventListener("click", (event) => {
      const button = event.target.closest(".doc-done");
      if (!button) return;

      const sec = button.closest(".doc-sec");
      if (!sec) return;

      if (done.has(sec.id)) {
        done.delete(sec.id);
      } else {
        done.add(sec.id);
      }
      write();
      syncProgress();
    });

    if (resetEl) {
      resetEl.addEventListener("click", () => {
        if (!window.confirm("Reset all lesson progress for this curriculum?")) return;
        done = new Set();
        write();
        syncProgress();
      });
    }

    syncProgress();
  }
})();
