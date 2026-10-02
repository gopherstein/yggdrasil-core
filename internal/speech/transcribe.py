"""Transcribe one audio file with faster-whisper, on this computer.

Reads a JSON config path from argv[1]: {"audio", "model", "models_dir",
"language"}. Prints one JSON object: {"language", "duration", "text",
"segments": [{"start", "end", "text"}]}.
"""
import json
import sys

from faster_whisper import WhisperModel

cfg = json.load(open(sys.argv[1]))
model = WhisperModel(cfg["model"], device="cpu", compute_type="int8", download_root=cfg["models_dir"])
segments, info = model.transcribe(cfg["audio"], language=cfg.get("language") or None, vad_filter=True)
out = []
for s in segments:
    out.append({"start": round(s.start, 2), "end": round(s.end, 2), "text": s.text.strip()})
json.dump({
    "language": info.language,
    "duration": round(info.duration, 2),
    "text": " ".join(s["text"] for s in out).strip(),
    "segments": out,
}, sys.stdout)
