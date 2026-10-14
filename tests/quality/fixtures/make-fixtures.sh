#!/usr/bin/env bash
# Makes the video fixture for the vision cases (#449): a red square moving
# left to right across a white picture for three seconds. It's drawn here,
# so it's our own; run it again to remake the file, with ffmpeg installed.
set -euo pipefail
cd "$(dirname "$0")"
ffmpeg -y -loglevel error \
  -f lavfi -i "color=c=white:s=480x270:d=3:r=12" \
  -f lavfi -i "color=c=red:s=60x60:d=3:r=12" \
  -filter_complex "[0][1]overlay=x='20+t*130':y=105:shortest=1,format=yuv420p" \
  -c:v libx264 -preset veryslow -crf 30 -movflags +faststart \
  red-square-moving.mp4
ls -l red-square-moving.mp4
