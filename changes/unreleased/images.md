### Added

- Images on this computer: `image.generate` makes an image from a description, and `image.edit` changes an image in the chat from an instruction, with stable-diffusion.cpp and FLUX.2 [klein] 4B. Image generation is set up once from the Tools page, which recommends a model for the computer's memory (5.2 GB, or 8.8 GB for more detail), downloads it with progress, and can stop, resume, and remove it (`/api/v1/images/setup`). Downloads are pinned and checked; prompts and images are not sent anywhere. It runs on macOS with Apple silicon, and on Linux and Windows on the CPU.
- Chats accept PNG and JPEG images, which are shown in the chat and can be edited. Images the assistant makes are shown too.
- An Images capability in profiles turns image generation on or off. Offline profiles keep it.

### Changed

- "Can you …?" answers no longer count a tool that is on but cannot run on this computer, and say what would make it work, such as setting up image generation.
