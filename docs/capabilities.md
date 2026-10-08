# Capabilities

A capability is the switch a person uses. Tools are what the model actually calls.

| Capability | What it does | Tools |
|------------|----------------|-------|
| Internet | Search and open public web pages | `internet.search`, `internet.open` |
| Files | Find, read, and write workspace files | `filesystem.search`, `filesystem.read`, `filesystem.write` |
| Shell | Run a command on this computer | `terminal` |
| Git | Inspect the repo, commit, and push | `git.status`, `git.diff`, `git.log`, `git.show`, `git.add`, `git.commit`, `git.push` |
| Files you can download | Make documents, spreadsheets, and code files | `files.create` |

Connected services (GitHub, Home Assistant) and MCP tool sources add their own tools. They are listed on the Tools screen and in each profile's tools.

Turning a capability on allows every tool in it. The model can use those tools without an extra prompt. An existing General Assistant that you have not customized is updated to that behavior on the next launch. A profile you already changed keeps your permissions, including Ask or Deny.

Profiles are the place to turn a capability on or off. Advanced mode still lists each tool and its permission: allow (automatic), ask, or deny.

## Add a capability

1. Add the id in `internal/tools/catalog.go`.
2. Register the tools that belong to it.
3. Add the same group in `web/src/features/profiles/capabilities.ts`.
4. Describe the default policies for General Assistant, Programming, and offline profiles.

The model system prompt is built only from tools whose policy is not deny, and each turn is offered only the tools it needs (see [Tools](tools.md#which-tools-a-turn-is-offered)). If Internet is off, the prompt does not mention the web.

## Capability inventory

The capability inventory is a different list: what Toskar can do right now, worked out from the installed models, online computers, enabled tools, and connected services. Diagnostics → **What Toskar can do** shows it, and a chat question such as "Can you generate an image?" is answered from it. `GET /api/v1/capabilities` returns it.

A tool that is on but cannot run here yet, such as image generation before it is set up, does not count as available; the tool's reason is listed with it (`unavailable`).

### Guided installation

When a missing ability can be installed, the inventory lists the setup (`setups`, and `setup` on the ability): what would be installed, its download size, and the computer it would run on. Today that is image and video generation, with the model recommended for the computer it would run on.

A chat request that needs it, such as "Make me an image of a Viking tree", is answered by Toskar instead of the model. The answer says what is missing and what would be installed, and the answer's `setup` (client contract 1.2) carries the offer, which the app shows as a card:
1. **Set up** starts the install.
2. The card shows the download as it goes.
3. When it is ready, the card sends the request again, so it is finished.

**On a paired computer:** the offer picks the best computer among this one and the online paired computers that can set it up (#153). A computer whose GPU would do the work comes first, then one with memory to spare, and this computer when they're equal. A computer without the memory or the disk space for the model, with a gigabyte to spare, isn't offered. A laptop that would make pictures on its CPU is offered the workstation instead: "I can set it up on Workstation: FLUX.2 [klein] 4B, 5.2 GB to download. Its GPU will do the work. Workstation has 120 GB free." The card names the computer and its free space, starts the setup there, and shows its download. Once it's ready, the request runs there through [tools on other computers](clustering.md#tools-on-other-computers). Downloading to another computer is still the person's choice on the card. A paired computer running an older Toskar isn't offered. Paired computers' setups are remembered for 30 seconds; a request that needs a setup asks them again first.

"Can you generate images?" gets the same card, without a request to finish. Offers appear only in chats in the app, and only when the chat's profile allows the tools the setup would add; an API caller gets the model's answer as before.
