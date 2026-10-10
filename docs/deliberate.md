# Deliberate: design

Status: design (#459). Nothing here has shipped.

Several models answer the same question independently, check each other's reasoning, and a judge writes the final answer, saying so where they still disagree. For a short answer, such as a number or one fact, the cheaper path is a vote. It's opt-in, and it's measured before Auto ever turns it on.

## What exists today

- **One orchestrator runs every strategy.** `simple.Orchestrator.Run` (`internal/orchestrator/builtin/simple/simple.go`) resolves the profile's strategy: single, planned, or team (`strategyOf`, `team.go`). It talks to the app only through `chatExecEnv` (`internal/app/chat.go`), the `pluginapi.ExecutionEnvironment` with `Generate`, `ExecuteTool`, `Emit`, and `NodeForRole`.
- **Roles already fan out across computers.** Team gives each part of a plan its own `worker:N` role (`spreadWorkers`). `NodeForRole` places each role in turn and passes the computers already used as `avoid` (`App.placeRoleWith`, `internal/app/cluster.go`). `modelForRole` maps `worker:2` to the profile's `worker` model (`profiles.RoleModel`). Workers run in parallel when the plan allows.
- **Team's reviewer** (`reviewAnswer`) checks a finished answer. **Thorough effort** has no reviewer: it raises the budget (`huginn.BudgetFor`: more pages, corrections, and tool calls) and adds `checkConsistency` to the checks every effort runs (figures, arithmetic, code, links; `verify.go`). None of these compare independent answers to the same question.
- **Stop** cancels the turn's context (`App.StopChat`), which every `Generate`, tool call, worker, and remote call shares; the partial answer is kept (`keepStopped`).
- **Steps and events:** orchestrator events go through `chatExecEnv.Emit` to the run trace, the answer's steps (`turnTrace.meta()` → `MessageMeta.Steps`), the API's progress callback, and the bus.
- **Gaps:**
  - `pluginapi.ChatRequest.Temperature` and `nodes.RemoteChatRequest.Temperature` exist, and a paired computer passes a remote request's temperature on (`internalChatStream`), but `generateOnNode`, which sends every chat call, never sets one.
  - No chat call sets `MaxTokens`, so there's no per-turn token budget.
  - `huginn.Classify` knows Chat, Current, Coding, Research, and Local, but not math, reasoning, or a single factual question.
  - The quality set matches answers by regex and has no category field.

## How a deliberated turn runs

Deliberate replaces the step where a strategy writes its answer. Planning, tools, lookups, and the checks before and after stay as they are. In the first version, it applies to single and planned turns; Team keeps its own planner, workers, and reviewer.

1. **Drafts.** Two or three drafters answer the same question, with the same context, without seeing each other. Each drafter is a role, `drafter:1`…`drafter:N`, so drafts are placed with `NodeForRole` exactly as workers are, and run at the same time on different paired computers when there are any. Each draft ends with a line `Final answer: …`.
2. **Short-answer vote.** When every draft's final answer is short (a number, a date, a name, yes or no, a few words), they're normalized and compared. If a majority agree, that's the answer: the agreeing draft with the clearest reasoning is kept, and nothing more runs. Most questions should stop here.
3. **Cross-examination.** Otherwise, and for long answers, each draft is critiqued against the others as structured output, `{claims, disagreements, likely_errors}`. The critique runs only when the drafts disagree.
4. **Reconcile.** A judge writes the final answer from the drafts and critiques. Where they still disagree, the answer says so and gives both sides, instead of picking one silently.

The drafts and the judge are ordinary `Generate` calls, so retries on another computer, fallback models, Stop, the run trace, and the checks all apply unchanged.

## Choosing the drafters and the judge

- **Different families first.** Small models make reasoning slips, and the same model checking itself tends to repeat them. Drafters are picked from installed models of different families (`contracts.Model.Family`) that fit in memory on the computers they'll run on (`huginn` scoring, as routing does). A Qwen draft beside a Llama or Mistral draft catches more than two Qwen drafts.
- **One model, several temperatures,** when only one family fits: drafts at different temperatures. This needs `Temperature` plumbed through `Generate` → `generateOnNode` → the local runtime and `RemoteChatRequest`.
- **The judge** is the profile's `reviewer` model when it has one, else the largest model that fits.
- **A profile can name them:** `drafter` and `judge` role models, like `worker` and `reviewer` today.

## Controls

- **Profile setting** `orchestration.deliberate`: `never` (the default), `always`, or `auto`. It goes in `contracts.OrchestrationPolicy`, validated in `profiles.ValidateOrchestration`, and shown in the profile's orchestration controls.
- **Auto** turns it on only for request kinds where the quality run shows a clear gain on the person's hardware class, and only when the computer (or cluster) has room for the extra drafts right now. This needs a new `Reasoning` kind in `huginn.Classify` (math word problems, logic, multi-step questions, trick questions), tested in `tests/quality/text.json`. Until the numbers are in, Auto behaves like `never`.
- **Budgets:**
  - At most three drafts.
  - A per-draft token cap (`MaxTokens`, newly plumbed).
  - Time: the profile's `TimeoutSeconds`, as for any turn.

  When the budget runs out, the best answer so far is kept (the vote's majority, or the first finished draft) with a notice in the steps. A deliberated turn never fails where a plain one would have answered.

## What people see

- **In the answer's steps:** "3 drafts · they agreed" or "3 drafts · 1 disagreement resolved", with each draft viewable: which model, on which computer, its final answer.
- **Events:** `deliberate.draft` (started and finished, per drafter), `deliberate.critique`, and `deliberate.done` (`{drafts, agreed, disagreements, outcome}`), through `Emit` like the others.
- **The answer's meta** gains a `deliberation` record with the drafts and the outcome, so the web app and the phone can show them. This is a client contract addition.
- **Stop** stops every draft, critique, and judge call at once.

## Measured before it's on

- **Reasoning cases** in the quality set: math word problems, logic puzzles, multi-hop facts, and trick questions, each with an exact expected answer, and a `kind` per case so results can be broken down.
- **The comparison:** each case runs as a single model, as a vote, and as full deliberation, per model and per model pair. The report gives accuracy, time, and tokens for each mode.
- **The matrix:** `quality.yml` already runs one model at a time. Pairs need two models loaded at once, inside the runner's memory and CPU caps. No quality run starts before the change that caps it is merged.
- **The Auto rule** is set from these numbers, per request kind and hardware class, and the numbers are published (the blog post: "Do two small models beat one big one?").

## Building it

1. This design.
2. Reasoning cases with `kind` and exact answers in the quality set, with time and tokens in the report, and a single-model baseline.
3. Plumbing: `Temperature` and `MaxTokens` through `Generate`, and the `drafter` and `judge` roles.
4. Drafts and the short-answer vote, behind `deliberate: always`, with events, steps, and the `deliberation` meta.
5. Cross-examination and the judge.
6. The `Reasoning` kind, the Auto rule from the measurements, and the controls and steps in the web app and the phone.
7. Docs, the changelog, and the blog post.
