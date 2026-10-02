### Fixed

- Cancelling training on a paired computer just as it starts now stops it there. Before, a cancel that arrived while the examples were still being sent could leave the paired computer training until its daemon restarted, holding its training slot. A cancel now waits for the paired computer to answer (up to 2 minutes), then stops the run there and removes its files.
