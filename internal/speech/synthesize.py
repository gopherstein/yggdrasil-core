"""Read text aloud with Piper, on this computer.

Reads a JSON config path from argv[1]: {"text", "voice", "voices_dir",
"out"}. Downloads the voice the first time it is used, writes a WAV file,
and prints {"seconds"}.
"""
import json
import os
import subprocess
import sys
import wave

from piper import PiperVoice

cfg = json.load(open(sys.argv[1]))
model = os.path.join(cfg["voices_dir"], cfg["voice"] + ".onnx")
if not os.path.exists(model):
    subprocess.run([sys.executable, "-m", "piper.download_voices", "--data-dir", cfg["voices_dir"], cfg["voice"]],
                   check=True, stdout=sys.stderr)
voice = PiperVoice.load(model)
with wave.open(cfg["out"], "wb") as wav:
    voice.synthesize_wav(cfg["text"], wav)
with wave.open(cfg["out"], "rb") as wav:
    seconds = wav.getnframes() / float(wav.getframerate())
json.dump({"seconds": round(seconds, 2)}, sys.stdout)
