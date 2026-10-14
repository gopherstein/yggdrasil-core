# Fixtures

Files the quality cases attach that are kept in the repository rather than
drawn by the test. Everything here is ours, made by the script beside it;
nothing is downloaded, and nothing shows a person or a brand.

| File | What it is | Made by |
| --- | --- | --- |
| `red-square-moving.mp4` | A red square moving left to right across a white picture, 3 seconds | `make-fixtures.sh` (ffmpeg) |

The pictures for the vision cases are drawn by the test itself
(`pictures_test.go` and `shapesPNG` in `files_test.go`), not kept here.
