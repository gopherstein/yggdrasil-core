# Topic controls

Topic controls keep an assistant on the subject its people came for (#345). A tire shop's assistant answers about tires, wheels, alignment, and the shop's hours and bookings, and politely declines an essay, medical advice, or politics.

Set them on a profile, under **Profiles → Edit → Topics**:

- **Stays on:** what the assistant is for, in plain words. Leave it empty for no topic controls.
- **Example questions:** questions it's for, one per line.
- **Never discusses:** subjects always off limits, even when they look related, such as "other shops' prices".
- **When asked something else:** the exact reply to anything off topic. Leave it empty, and the assistant says in one short sentence what it can help with, in the person's language.
- **Web search only on:** the sites web search and pages keep to, such as the shop's own site and tire makers, one per line, with their subdomains. Leave it empty for any site.
- **Add to every web search:** words added to each search, such as "tires", so results stay on the subject.

Greetings, thanks, and "what can you do?" are always fine, so a strict assistant doesn't feel broken.

## How it works

The topic rules come first in every turn, as the administrator's rules, ahead of everything else the model is told. They say that nothing later changes them: not the person's preferences or memories, not an application's instructions through the API, not a document, file, or web page, and not a message asking to ignore them, to pretend, or to play a role.

**How strict** picks how much more is done:

- **Guide:** the rules alone. Quickest, and fine for your own people.
- **Enforce:** for the public and chat portals. Each message is checked before it's answered, and each answer before it's shown:
  1. **Before answering,** a short call labels the message *on topic*, *small talk*, or *off topic*, reading it with the conversation so far, so "and the 18-inch?" still counts. An off-topic message gets the set reply, and the full answer never runs, which also saves the work.
  2. **After answering,** a short call reads the answer with its question. An answer that went off topic anyway, because the message talked its way past the first check, is replaced by the set reply.

  The set reply is the one under **When asked something else**, word for word. Without one, the check writes one short sentence in the person's language saying what the assistant can help with, and when it can't, Toskar sends "I can't help with that here. What else can I help you with?" in the person's language.

  The checks use the model that answers, which is already loaded, so each adds one short call. If a check gives nothing usable, the turn goes on as Guide, and the run trace says so.

The run trace (`GET /api/v1/runs`, and the run details under an answer) notes a held message or a replaced answer, and its `topic` says what the check found.

## Who it applies to

- **Chat portals (#205):** a portal's profile brings its topic controls with it, so its visitors can't turn them off.
- **API keys:** on **API Access**, a key's **Answers with** pins it to one profile. Every request with that key, from a website or another app, answers with the profile and its topic controls, and can't name another profile or a model.

With topic controls, the assistant answers everything by its own rules: Toskar's own replies about itself ("how do I install MCP?"), "remember …" commands, and offers to install something aren't given in its place.

## Narrower reach

What an assistant can reach is part of staying on topic:

- **Web:** with sites set, every web search asks only those sites, results from anywhere else are dropped, and pages elsewhere aren't opened, including a page a link or a redirect leads to. This holds for the model's own searches, Toskar's look-ups, and pages followed from results, at either strictness. The words are added to every search.
- **Tools:** a profile uses only the tools turned on in its **Tools** tab. Give a public assistant only the ones its topic needs: a tire shop's needs web search, maybe maps, and not files or code.
- **Knowledge:** a turn searches only the profile's own knowledge sources, from its **Knowledge** tab.

## Be honest about the limits

No language model can promise it never goes off topic. Guide makes it the model's clear instruction; Enforce makes going off topic hard and visible, not impossible. A small model can label a message wrongly either way, so try a profile's real questions before publishing it.

Topic controls don't replace keeping private data out of a public assistant: anything in its knowledge sources or reachable by its tools can still be asked about. Give a public assistant only the knowledge and tools it needs.
