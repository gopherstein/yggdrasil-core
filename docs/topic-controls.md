# Topic controls

Topic controls keep an assistant on the subject its people came for (#345). A tire shop's assistant answers about tires, wheels, alignment, and the shop's hours and bookings, and politely declines an essay, medical advice, or politics.

Set them on a profile, under **Profiles → Edit → Topics**:

- **Stays on:** what the assistant is for, in plain words. Leave it empty for no topic controls.
- **Example questions:** questions it's for, one per line.
- **Never discusses:** subjects always off limits, even when they look related, such as "other shops' prices".
- **When asked something else:** the exact reply to anything off topic. Leave it empty, and the assistant says in one short sentence what it can help with, in the person's language.

Greetings, thanks, and "what can you do?" are always fine, so a strict assistant doesn't feel broken.

## How it works

The topic rules come first in every turn, as the administrator's rules, ahead of everything else the model is told. They say that nothing later changes them: not the person's preferences or memories, not an application's instructions through the API, not a document, file, or web page, and not a message asking to ignore them, to pretend, or to play a role.

This is **Guide**, the rules alone. It's cheap, and fine for staff. **Enforce**, for the public and chat portals, adds a check before answering and a check on the answer; it comes next.

A chat portal (#205) using a profile brings its topic controls with it, so its visitors can't turn them off.

## Be honest about the limits

No language model can promise it never goes off topic. Guide makes it the model's clear instruction; Enforce will make going off topic hard and visible, not impossible.

Topic controls don't replace keeping private data out of a public assistant: anything in its knowledge sources or reachable by its tools can still be asked about. Give a public assistant only the knowledge and tools it needs.
