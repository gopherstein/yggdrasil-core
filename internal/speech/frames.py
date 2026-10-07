"""Sample frames from a video with PyAV, on this computer, for a model that
can see (#191).

Reads a JSON config path from argv[1]: {"video", "count", "side",
"out_dir"}. Writes frame-<n>.jpg files to out_dir, evenly spaced through the
clip and at most side pixels on the longer side, and prints one JSON object:
{"duration", "has_audio", "frames": [{"time", "file"}]}.
"""
import json
import os
import sys
from fractions import Fraction

import av

cfg = json.load(open(sys.argv[1]))
count = max(1, int(cfg["count"]))
side = int(cfg["side"])


def jpeg(frame):
    """Encodes a frame as a JPEG, smaller when it is large."""
    scale = min(1.0, side / max(frame.width, frame.height))
    w = max(2, int(frame.width * scale) // 2 * 2)
    h = max(2, int(frame.height * scale) // 2 * 2)
    img = frame.reformat(width=w, height=h, format="yuvj420p")
    img.pts = 0
    enc = av.CodecContext.create("mjpeg", "w")
    enc.width, enc.height, enc.pix_fmt = w, h, "yuvj420p"
    enc.time_base = Fraction(1, 25)
    packets = list(enc.encode(img)) + list(enc.encode(None))
    return b"".join(bytes(p) for p in packets)


with av.open(cfg["video"]) as container:
    if not container.streams.video:
        sys.exit("the file has no video")
    stream = container.streams.video[0]
    has_audio = bool(container.streams.audio)
    stream.thread_type = "AUTO"
    duration = 0.0
    if container.duration:
        duration = container.duration / av.time_base
    elif stream.duration and stream.time_base:
        duration = float(stream.duration * stream.time_base)
    # The middle of each equal part, so a fade at either end is skipped.
    times = [duration * (i + 0.5) / count for i in range(count)] if duration > 0 else [0.0]
    frames = []
    for i, t in enumerate(times):
        if t > 0 and stream.time_base:
            container.seek(int(t / stream.time_base), stream=stream, backward=True)
        picked = None
        for frame in container.decode(stream):
            picked = frame
            if frame.time is None or frame.time >= t - 0.04:
                break
        if picked is None:
            continue
        name = "frame-%d.jpg" % i
        with open(os.path.join(cfg["out_dir"], name), "wb") as f:
            f.write(jpeg(picked))
        frames.append({"time": round(picked.time if picked.time is not None else t, 2), "file": name})

json.dump({"duration": round(duration, 2), "has_audio": has_audio, "frames": frames}, sys.stdout)
