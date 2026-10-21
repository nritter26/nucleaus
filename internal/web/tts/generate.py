"""Generate curriculum TTS audio with Microsoft Edge TTS (Ryan voice).

Reads the five revised curriculum markdown files, strips non-prose
markdown, and synthesizes one mp3 per `##` section into
internal/web/tts/<slug>/<NN-section-slug>.mp3 plus a manifest.json.

Usage:
    python3 generate.py            # generate missing files only
    python3 generate.py --force    # regenerate everything
    python3 generate.py --track go # single track

The manifest.json is consumed by internal/web/tts.go at build time
(embedded), while the mp3s are served from disk on demand.
"""

from __future__ import annotations

import argparse
import asyncio
import hashlib
import json
import re
import sys
from pathlib import Path

import edge_tts

ROOT = Path(__file__).resolve().parent
CURRICULUM = ROOT.parent.parent.parent / "docs" / "curriculum-room"

VOICE = "en-GB-RyanNeural"
RATE = "+0%"
OUT_SHARED = {
    "go": ("golang-detailed-curriculum-revised.md", "Go"),
    "typescript": ("typescript-detailed-curriculum-revised.md", "TypeScript"),
    "postgres": ("pg_curriculum.md", "Postgres"),
    "docker": ("docker_curriculum.md", "Docker"),
    "rust": ("rust_curriculum_revised.md", "Rust"),
}


def clean(md: str) -> str:
    """Markdown -> spoken prose."""
    text = md
    # drop fenced blocks and tables, replace with a hint
    text = re.sub(r"```.*?```", " A syntax example follows in the written notes. ", text, flags=re.S)
    text = re.sub(r"^\s*\|.*\|\s*$", " A reference table follows in the written notes. ", text, flags=re.M)
    # links: keep label only; images are dropped
    text = re.sub(r"!\[[^\]]*\]\([^)]*\)", "", text)
    text = re.sub(r"\[([^\]]*)\]\([^)]*\)", r"\1", text)
    # headings -> "N. Heading" already given by caller prose cue
    text = re.sub(r"^[#>\s]+", "", text, flags=re.M)
    # markup noise
    text = re.sub(r"[*_`]+", "", text)
    text = re.sub(r"^\s*[-*+]\s+", lambda m: "bullet: ", text, flags=re.M)
    text = re.sub(r"^\s*\d+\.\s+", lambda m: f"point {m.group(0).strip()} ", text, flags=re.M)
    text = re.sub(r"^\s*-\s\[[ xX]\]\s*", "", text, flags=re.M)
    # fragment URLs read terribly: scrub common http strings
    text = re.sub(r"https?://\S+", "", text)
    text = re.sub(r"\s+", " ", text)
    return text.strip()


def sections(track: str) -> list[tuple[str, str]]:
    """(heading, spoken_text) pairs, skipping any empty bodies."""
    fname, label = OUT_SHARED[track]
    raw = (CURRICULUM / ({"go": "go", "typescript": "ts", "postgres": "pg",
                          "docker": "docker", "rust": "rust"}[track]) / fname).read_text()
    parts = re.split(r"\n+## ", raw)
    out = []  # (heading, cleaned body, spoken_text) needs no file prefix yet
    for part in parts[1:]:  # parts[0] is the front matter before the first ##
        part = part.strip()
        if not part:
            continue
        pieces = re.split(r"\n", part, maxsplit=1)
        heading = pieces[0].strip().lstrip("#").strip()
        body = pieces[1].strip() if len(pieces) > 1 else ""
        if heading.lower() == "table of contents":
            continue
        spoken = clean(body)
        if len(spoken.split()) < 12:
            continue
        label = OUT_SHARED[track][1]
        prose = clean(heading)
        prefix = f"{label}. " if not out else ""
        text = prefix + f"Section: {prose}. " + spoken
        out.append((heading, text))
    return out


def slugify(s: str) -> str:
    s = re.sub(r"[^\w\s-]", "", s.lower())
    s = re.sub(r"[\s_]+", "-", s).strip("-")
    h = hashlib.sha1(s.encode()).hexdigest()[:6]
    return (s[:42] or "section") + "-" + h


async def synth(text: str, out: Path, sem: asyncio.Semaphore) -> bool:
    async with sem:
        for attempt in range(3):
            try:
                com = edge_tts.Communicate(text, VOICE, rate=RATE)
                await com.save(out)
                return True
            except Exception as exc:  # noqa: BLE001
                if attempt == 2:
                    print(f"FAIL {out.name}: {exc}", file=sys.stderr)
                    return False
                await asyncio.sleep(2 * (attempt + 1))
        return False


async def run(track: str, force: bool, concurrency: int) -> list[dict]:
    outdir = ROOT / track
    outdir.mkdir(parents=True, exist_ok=True)
    sec_list = sections(track)
    sem = asyncio.Semaphore(concurrency)
    tasks = []
    plan = []
    for i, (heading, text) in enumerate(sec_list):
        name = f"{i + 1:03d}-{slugify(heading)}.mp3"
        target = outdir / name
        if force or not target.exists() or target.stat().st_size == 0:
            tasks.append(asyncio.create_task(synth(text, target, sem)))
        else:
            tasks.append(asyncio.sleep(0))  # keep indices aligned
        plan.append({
            "n": i + 1,
            "title": heading,
            "file": f"{track}/{name}",
        })
    results = await asyncio.gather(*tasks)
    items = []
    for p, ok in zip(plan, results):
        target = ROOT / p["file"]
        if ok is True or target.exists() and target.stat().st_size > 0:
            items.append(p)
    return items


async def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--track", choices=[*OUT_SHARED, "all"], default="all")
    ap.add_argument("--force", action="store_true")
    ap.add_argument("--workers", type=int, default=4)
    args = ap.parse_args()

    tracks = list(OUT_SHARED) if args.track == "all" else [args.track]
    manifest = {"voice": VOICE, "tracks": {}}
    for track in tracks:
        print(f"track {track} …", flush=True)
        items = await run(track, args.force, args.workers)
        manifest["tracks"][track] = {"name": OUT_SHARED[track][1], "items": items}
    out = ROOT / "manifest.json"
    out.write_text(json.dumps(manifest, indent=1))
    total = sum(len(v["items"]) for v in manifest["tracks"].values())
    print(f"manifest {out} tracks={len(tracks)} items={total}")


if __name__ == "__main__":
    asyncio.run(main())
