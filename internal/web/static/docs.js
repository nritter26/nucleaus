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
})();
