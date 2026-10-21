// tts.js — curriculum read-aloud player controls.
// Speed + volume persist in localStorage; the time bar scrubs the
// section; prev/next/back/fwd buttons jump between sections.
(function () {
  "use strict";

  var root = document.getElementById("tts-root");
  if (!root) return;

  var audio = document.getElementById("tts-audio");
  var play = document.getElementById("tts-play");
  var seek = document.getElementById("tts-seek");
  var curEl = document.getElementById("tts-time-cur");
  var totalEl = document.getElementById("tts-time-total");
  var rate = document.getElementById("tts-rate");
  var vol = document.getElementById("tts-vol");
  var volVal = document.getElementById("tts-vol-val");
  var prev = document.getElementById("tts-prev");
  var next = document.getElementById("tts-next");
  var back = document.getElementById("tts-back");
  var fwd = document.getElementById("tts-fwd");
  var secN = document.getElementById("tts-sec-n");
  var secTitle = document.getElementById("tts-sec-title");
  var trackRow = document.getElementById("tts-tracks");

  var sections = [];
  var current = -1;
  var autoAdvance = false;

  function ls(key, def) {
    try { return localStorage.getItem(key) || def; } catch (err) { return def; }
  }
  function lsSet(key, val) {
    try { localStorage.setItem(key, val); } catch (err) { /* ignore */ }
  }

  function fmt(sec) {
    if (!isFinite(sec)) return "0:00";
    sec = Math.max(0, Math.floor(sec));
    var m = Math.floor(sec / 60);
    var s = sec % 60;
    return m + ":" + (s < 10 ? "0" : "") + s;
  }

  function highlight(srcButton) {
    document.querySelectorAll(".tts-sec.is-current").forEach(function (el) {
      el.classList.remove("is-current");
    });
    if (srcButton) srcButton.classList.add("is-current");
  }

  function load(index, autoplay) {
    if (index < 0 || index >= sections.length) return;
    current = index;
    var s = sections[index];
    secN.textContent = String(s.n);
    secTitle.textContent = s.title + " · " + s.trackName;
    audio.src = s.src;
    audio.playbackRate = parseFloat(rate.value) || 1;
    if (autoplay) {
      autoAdvance = true;
      audio.play().catch(function () { /* autoplay blocked */ });
    }
    highlight(s.button);
  }

  function playToggle() {
    if (!audio.src) {
      load(0, true);
      return;
    }
    if (audio.paused) {
      autoAdvance = true;
      audio.play().catch(function () { /* ignore */ });
    } else {
      audio.pause();
    }
  }

  // Playlist pieces: keep every visible section list, remember the parent
  // track for the "now playing" label.
  document.querySelectorAll(".tts-sec").forEach(function (button, i) {
    sections.push({
      n: button.getAttribute("data-n"),
      track: button.getAttribute("data-track"),
      trackName: (button.closest("[data-track-list]") || {}).dataset ?
        button.closest("[data-track-list]").getAttribute("data-track-list") : "",
      src: button.getAttribute("data-src"),
      title: button.querySelector(".tts-sec-name").textContent,
      button: button,
      index: i
    });
    button.addEventListener("click", function () {
      load(buttonStateIndex(button), true);
    });
  });

  function buttonStateIndex(button) {
    for (var i = 0; i < sections.length; i++) {
      if (sections[i].button === button) return i;
    }
    return -1;
  }

  play.addEventListener("click", playToggle);

  audio.addEventListener("play", function () {
    play.textContent = "\u23F8";
    play.setAttribute("aria-label", "Pause");
  });
  audio.addEventListener("pause", function () {
    play.textContent = "\u25B6";
    play.setAttribute("aria-label", "Play");
  });

  audio.addEventListener("loadedmetadata", function () {
    totalEl.textContent = fmt(audio.duration);
  });

  audio.addEventListener("timeupdate", function () {
    curEl.textContent = fmt(audio.currentTime);
    if (audio.duration > 0) {
      seek.value = String(Math.round((audio.currentTime / audio.duration) * 1000));
    }
  });

  audio.addEventListener("ended", function () {
    if (autoAdvance && current < sections.length - 1) {
      load(current + 1, true);
    }
  });

  seek.addEventListener("input", function () {
    if (audio.duration > 0) {
      audio.currentTime = (parseFloat(seek.value) / 1000) * audio.duration;
      curEl.textContent = fmt(audio.currentTime);
    }
  });

  back.addEventListener("click", function () {
    audio.currentTime = Math.max(0, audio.currentTime - 10);
  });
  fwd.addEventListener("click", function () {
    if (audio.duration) audio.currentTime = Math.min(audio.duration, audio.currentTime + 10);
  });

  prev.addEventListener("click", function () {
    if (current > 0) {
      load(current - 1, current >= 0 ? true : false);
    }
  });
  next.addEventListener("click", function () {
    if (current < sections.length - 1) {
      load(current + 1, current >= 0 ? true : false);
    }
    if (current === -1) load(0, true);
  });

  rate.addEventListener("change", function () {
    audio.playbackRate = parseFloat(rate.value) || 1;
    lsSet("tts-rate", rate.value);
  });
  rate.value = ls("tts-rate", "1");
  audio.playbackRate = parseFloat(rate.value) || 1;

  function applyVolume() {
    audio.volume = parseInt(vol.value, 10) / 100;
    volVal.textContent = String(vol.value);
  }
  vol.addEventListener("input", applyVolume);
  vol.value = ls("tts-vol", "100");
  applyVolume();

  // Track selector switches the visible playlist.
  trackRow.addEventListener("click", function (event) {
    var button = event.target.closest(".tts-track");
    if (!button) return;
    trackRow.querySelectorAll(".tts-track").forEach(function (t) {
      var isOne = t === button;
      t.classList.toggle("is-active", isOne);
      t.setAttribute("aria-selected", isOne ? "true" : "false");
    });
    document.querySelectorAll("[data-track-list]").forEach(function (list) {
      list.hidden = list.getAttribute("data-track-list") !== button.getAttribute("data-track");
    });
    highlight(null);
    if (audio.src) {
      audio.pause();
      secTitle.textContent = "Choose a section to listen to";
      secN.textContent = "\u2014";
    }
  });
})();
