### Added

- Image and video generation use the GPU on Linux and Windows. With a
  graphics card that has a Vulkan driver (NVIDIA, AMD, or Intel), setup
  installs stable-diffusion.cpp's Vulkan build, so a picture takes seconds
  instead of minutes. A Vulkan build that can't start on the GPU falls back
  to the CPU build by itself. The Tools page says which build makes them,
  with a button to switch. An existing setup on the CPU build offers **Use
  the GPU build** there.
- Paired computers that make pictures on the GPU are now reported as
  accelerated, so heavy image work goes to them.
