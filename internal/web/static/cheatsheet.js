/* Mallard Server Hub — cheat sheet search, doc tabs and copy buttons */
(() => {
  "use strict";

  const search = document.getElementById("cheat-search");
  const tabs = document.getElementById("cheat-tabs");
  const countEl = document.getElementById("cheat-count");
  const noneEl = document.getElementById("cheat-none");
  const docs = Array.from(document.querySelectorAll(".cheat-doc"));

  if (!search || !docs.length) return;

  let activeDoc = "all";

  const entries = Array.from(document.querySelectorAll(".cheat-entry")).map((entry) => ({
    el: entry,
    hay: (entry.dataset.search || "").toLowerCase(),
  }));

  const apply = () => {
    const query = search.value.trim().toLowerCase();
    let visible = 0;

    entries.forEach((entry) => {
      const doc = entry.el.closest(".cheat-doc");
      const docMatch = activeDoc === "all" || (doc && doc.dataset.doc === activeDoc);
      const textMatch = !query || entry.hay.includes(query);
      const show = docMatch && textMatch;
      entry.el.hidden = !show;
      if (show) visible += 1;
    });

    docs.forEach((doc) => {
      let docVisible = 0;
      doc.querySelectorAll(".cheat-section").forEach((section) => {
        const shown = section.querySelectorAll(".cheat-entry:not([hidden])").length;
        section.hidden = shown === 0;
        docVisible += shown;
      });
      doc.hidden = docVisible === 0;
    });

    if (countEl) {
      countEl.textContent = `${visible} command${visible === 1 ? "" : "s"}`;
    }
    if (noneEl) noneEl.hidden = visible !== 0;
  };

  search.addEventListener("input", apply);

  if (tabs) {
    tabs.addEventListener("click", (event) => {
      const button = event.target.closest("button[data-doc]");
      if (!button) return;

      activeDoc = button.dataset.doc;
      tabs.querySelectorAll("button").forEach((tab) => tab.classList.toggle("is-active", tab === button));
      apply();

      const target = activeDoc === "all" ? docs[0] : document.querySelector(`.cheat-doc[data-doc="${activeDoc}"]`);
      if (target && !target.hidden) {
        target.scrollIntoView({ behavior: "smooth", block: "start" });
      }
    });
  }

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
      const code = button.closest(".cheat-cmd")?.querySelector("code");
      if (!code) return;

      const ok = await copyText(code.textContent);
      button.textContent = ok ? "copied" : "failed";
      button.classList.toggle("is-done", ok);

      window.setTimeout(() => {
        button.textContent = "copy";
        button.classList.remove("is-done");
      }, 1400);
    });
  });

  document.addEventListener("keydown", (event) => {
    if (event.key === "/" && document.activeElement !== search) {
      event.preventDefault();
      search.focus();
      search.select();
    }
  });
})();
