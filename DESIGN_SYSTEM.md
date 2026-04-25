# Chalk Console UX Design System

A UX design system for console output in task-based CLI agents. Each ADR defines a user-facing output pattern: what the user sees, why it is shaped that way, and how to implement it correctly with `github.com/fogfish/chalk`.

---

## Vocabulary

| Term              | Meaning                                                                  |
| ----------------- | ------------------------------------------------------------------------ |
| **Task**          | A named unit of work shown as a single progress line                     |
| **Sub-task**      | A task nested inside another; indented one level deeper                  |
| **Spinner**       | The animated braille character showing a task is in progress             |
| **Wall clock**    | Absolute elapsed time since the agent started (`00m 00.0s`)              |
| **Duration**      | Time a single task took (`00.0s`) shown on completion                    |
| **Suffix**        | A short parenthetical annotation appended to a `Done` line               |
| **Note**          | Multi-line text printed beneath a task via `Printf`                      |
| **Run separator** | A decorative line marking the boundary between progress and final output |

---

## ADR-001: Task Label Language

**Status:** Accepted

**Context:**
Task labels are the primary text the user reads while the agent runs. Inconsistent verb forms (gerund vs imperative vs past tense) produce a jagged, unprofessional feel.

**Decision:**
Write all task labels in **title-cased gerund form** — the action as it is happening. Never use past tense or imperative.

```
✓  "Loading dataset"        — gerund, describes ongoing action
✓  "Validating schema"
✓  "Generating embeddings"
✗  "Load dataset"           — imperative
✗  "Dataset loaded"         — past tense
✗  "load_dataset"           — snake_case
✗  "LOAD"                   — shouting, too terse
```

For tasks that operate on a specific named artefact, include it in the label:

```go
chalk.Task(ctx, "Processing %s", path)
chalk.Task(sub, "Parsing chunk %d of %d", i+1, total)
```

**Consequences:**
- The spinner line reads as a natural sentence: `⠹ Loading dataset`
- The done line reads as a completed action: `✓ Loading dataset`
- Labels stay consistent whether displayed by the spinner or by the log printer in CI.

---

## ADR-002: Visual Hierarchy — When to Use Sub-Tasks

**Status:** Accepted

**Context:**
Flat output with everything at one level gives no sense of structure. Deeply nested output with trivial leaf tasks creates noise. The hierarchy must match the user's mental model of the work.

**Decision:**
Use **at most two levels of nesting** in normal operation. Reserve a third level for exceptional diagnostic detail only.

Apply sub-tasks when:
- A top-level task spans **multiple distinct phases** that each take user-perceptible time (> 1 s).
- A phase **iterates over a collection** where individual item progress is meaningful to the user.
- An operation can **fail independently** per item and the user needs to see which item failed.

Do not create sub-tasks when:
- The operation completes in under one second.
- The sub-task label would duplicate the parent label.
- The only information is a count that fits in a Done suffix.

```
✓  Correct — two meaningful levels:

   ▶ Indexing documents
       ✓ Parsing     (0.3s)
       ✓ Chunking    (1.2s)
       ✓ Embedding   (8.4s)
   ✓ Indexing documents  (10.1s)

✗  Incorrect — spurious nesting:

   ▶ Indexing documents
       ▶ Starting indexing
           ▶ Calling function
   ✓ Indexing documents
```

**Consequences:**
- The user can see at a glance which phase is running without scrolling.
- Two levels map cleanly onto `ctx` (level 0) and `chalk.Sub(ctx)` (level 1).

---

## ADR-003: Done Suffix — Communicating Outcomes at a Glance

**Status:** Accepted

**Context:**
When a task completes, the user often needs one key metric — a count, a rate, a size — without reading a separate line. The Done line already carries timing; it can carry one additional scalar.

**Decision:**
Add a parenthetical suffix to `chalk.Done` for **exactly one key outcome metric**. Format: `"(noun value)"` or `"(value unit)"`. Keep it under 30 characters. Use it only when the metric is meaningful to the user; omit it for tasks where completion is self-evident.

```go
// quantity outcomes
chalk.Done(fmt.Sprintf("(%d records)", n))
chalk.Done(fmt.Sprintf("(%d of %d processed)", ok, total))

// performance outcomes
chalk.Done(fmt.Sprintf("(%.1f docs/s)", rate))

// state outcomes
chalk.Done("(cached)")
chalk.Done("(skipped — already exists)")
```

Never use the suffix for error-like information — that belongs in `Fail` or `Printf`.

```
✓  00m 12.4s  (0.3s) ✓ Embedding documents  (1 024 chunks)
✗  00m 12.4s  (0.3s) ✓ Embedding documents  (1024 chunks embedded successfully with no errors)
```

**Consequences:**
- The Done line conveys timing + outcome in one scan. The user does not need to search for a separate log line.
- Long explanations bloating the Done line destroy the visual rhythm; they belong in a `Printf` note.

---

## ADR-004: Notes — Surfacing Detail Without Noise

**Status:** Accepted

**Context:**
Some tasks produce detail that does not fit a single metric: warnings, per-item diagnostics, intermediate results, explanatory context. This information must be visible without overwhelming the task tree.

**Decision:**
Use `chalk.Printf` to emit **notes beneath the current task**. A note is indented one level deeper than its parent task. Emit notes only when the information is **actionable or diagnostic** — not for routine confirmation.

```go
chalk.Task(sub, "Validating schema")
for _, w := range warnings {
    chalk.Printf("⚠ %s", w)           // actionable: user may need to fix this
}
chalk.Done(fmt.Sprintf("(%d warnings)", len(warnings)))

// NOT this — purely confirmatory, adds no value:
chalk.Task(sub, "Connecting to database")
chalk.Printf("Connected successfully")  // ✗ the ✓ line already says this
chalk.Done()
```

Format notes as complete sentences or structured key: value pairs. Do not use notes as a substitute for a missing sub-task.

**Consequences:**
- Notes appear indented beneath the spinner in TTY mode, and as `slog.Info` lines in CI — both are scannable.
- Emitting notes only for actionable content keeps the output dense with signal, not noise.

---

## ADR-005: Run Separator — Marking the Boundary Between Progress and Output

**Status:** Accepted

**Context:**
When an agent produces a human-readable summary or final result after all tasks complete, the boundary between the progress tree and the result is ambiguous without a visual break.

**Decision:**
Emit a full-width Unicode separator line followed by a blank line before any final result block. Use the `━` character (U+2501) repeated to 66 columns. The final result block ends with a second blank line.

```go
chalk.Printf("\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
chalk.Printf("Complete!\n\n")
```

Use this pattern **once per run**, after all task reporting is finished. Do not use separators between phases within a run — task hierarchy conveys phase boundaries.

**Consequences:**
- The user immediately identifies where "what happened" ends and "what the result is" begins.
- A single separator preserves the visual weight; multiple separators dilute it.

---

## ADR-006: Error Messages — Precision and Actionability

**Status:** Accepted

**Context:**
When a task fails, the error message is the only signal the user has to understand what went wrong and what to do next. Terse or overly technical messages leave the user without a path forward.

**Decision:**
Error messages passed to `chalk.Fail` must answer three questions in order:

1. **What** failed — the specific object or operation, not the generic task label.
2. **Why** it failed — the root cause in plain language.
3. **What to do** — if there is a remediation action the user can take.

```go
// ✓ Specific, causal, actionable
chalk.Fail(fmt.Errorf("file %q: line 42: unknown field \"embedding_model\"; valid fields are: model, temperature, max_tokens", path))

// ✓ Specific and causal (no user action needed — system error)
chalk.Fail(fmt.Errorf("embedding API: rate limit exceeded after %d retries; retry after 60 s", retries))

// ✗ Generic — tells the user nothing
chalk.Fail(fmt.Errorf("processing failed"))

// ✗ Stack trace dump — not a user message
chalk.Fail(fmt.Errorf("%v", err))
```

Wrap errors with context at each layer using `fmt.Errorf("stage: %w", err)` so the final message has a causal chain without exposing Go internals.

**Consequences:**
- The chalk renderer wraps the error text at 80 columns under the failed task line — messages must be coherent prose, not stack traces.
- In CI log mode the error appears as a structured `slog.Error` field — the same message rules apply.

---

## ADR-007: Timing Perception — Designing for the Waiting User

**Status:** Accepted

**Context:**
The spinner and wall-clock timer are always visible. If a task runs for 30 seconds with no visible sub-task progress, the user cannot distinguish a working agent from a hung one.

**Decision:**
Decompose any task expected to take **longer than 5 seconds** into sub-tasks so the spinner advances and the user sees forward motion. If a task is inherently atomic (a single API call), emit a `Printf` note after a threshold to confirm progress.

```go
// Long atomic operation — emit a progress note after threshold
chalk.Task(sub, "Uploading to S3")
if err := upload(ctx, data, func(pct int) {
    if pct == 50 {
        chalk.Printf("50%% uploaded (%s transferred)", humanize.Bytes(transferred))
    }
}); err != nil {
    chalk.Fail(err)
    return err
}
chalk.Done(fmt.Sprintf("(%s)", humanize.Bytes(size)))

// Iterative operation — sub-task per item provides natural progress
chalk.Task(ctx, "Embedding %d chunks", len(chunks))
sub := chalk.Sub(ctx)
for i, chunk := range chunks {
    chalk.Task(sub, "Chunk %d", i+1)
    vec, err := embed(chunk)
    if err != nil {
        chalk.Fail(err)
        return err
    }
    chalk.Done()
}
chalk.Done(fmt.Sprintf("(%d vectors)", len(chunks)))
```

**Rule of thumb:**
| Expected duration | Pattern                                |
| ----------------- | -------------------------------------- |
| < 1 s             | Single task, no sub-tasks, no notes    |
| 1–5 s             | Single task with Done suffix           |
| 5–30 s            | Sub-tasks or progress note at midpoint |
| > 30 s            | Sub-tasks are mandatory                |

**Consequences:**
- The user maintains confidence the agent is working, not hung.
- Sub-tasks at this granularity also localise failures precisely.

---

## ADR-008: Iteration Output — Balancing Completeness and Scroll

**Status:** Accepted

**Context:**
Agents processing collections (files, chunks, API pages) can emit one sub-task line per item. For large collections this floods the terminal with hundreds of lines that scroll past unreadably.

**Decision:**
Apply the following thresholds for collection iteration output:

| Collection size | Output strategy                                                               |
| --------------- | ----------------------------------------------------------------------------- |
| ≤ 10 items      | One sub-task per item                                                         |
| 11–100 items    | One sub-task per item, but only emit Done suffix on failure or notable result |
| > 100 items     | Group into batches of 10–25; one sub-task per batch                           |
| > 1 000 items   | Single parent task with a progress note every 10% or fixed interval           |

```go
// ≤ 10 items — full detail
for i, doc := range docs {
    chalk.Task(sub, "Document %d: %s", i+1, doc.Name)
    chalk.Done(fmt.Sprintf("(%d tokens)", doc.TokenCount))
}

// > 100 items — batched
batchSize := 25
for start := 0; start < len(docs); start += batchSize {
    end := min(start+batchSize, len(docs))
    chalk.Task(sub, "Documents %d–%d", start+1, end)
    for _, doc := range docs[start:end] { /* process */ }
    chalk.Done(fmt.Sprintf("(%d processed)", end-start))
}

// > 1 000 items — interval notes
chalk.Task(ctx, "Processing %d documents", len(docs))
for i, doc := range docs {
    process(doc)
    if (i+1) % 100 == 0 {
        chalk.Printf("%d / %d complete", i+1, len(docs))
    }
}
chalk.Done(fmt.Sprintf("(%d documents)", len(docs)))
```

**Consequences:**
- The terminal does not scroll the task tree off screen for large collections.
- The user always sees a ratio or count that communicates how much work remains.

---

## ADR-009: Phase Structure — Top-Level Task Granularity

**Status:** Accepted

**Context:**
The top-level task list is the "table of contents" of the run. Too few tasks (one task for the entire run) gives no progress signal. Too many (one task per file in a 10 000-file batch) is noise.

**Decision:**
Structure top-level tasks around **semantic phases of the pipeline**, not around data items. Each phase should have a name that matches a concept in the problem domain.

```
✓  Good phase structure for a RAG ingestion agent:
   ▶ Loading documents
   ▶ Chunking text
   ▶ Generating embeddings
   ▶ Indexing vectors
   ▶ Writing results

✗  Anti-patterns:
   ▶ Processing                         — too generic
   ▶ Step 1 / Step 2 / Step 3          — not semantic
   ▶ file_001.txt / file_002.txt / …   — data, not phases
```

Each top-level task should encompass work that:
- Has a clear start and end from the user's perspective.
- Can succeed or fail independently of the other phases.
- Takes enough time to warrant a separate progress line (> 1 s in a typical run).

**Consequences:**
- The user can identify which phase the agent is in without understanding the code.
- Failure attribution is immediate: the ✗ line names the phase that broke.

---

## ADR-010: Cached Step Signalling

**Status:** Accepted

**Context:**
When the agent replays a run using the checkpoint cache, some tasks complete instantly. Without visual differentiation the user cannot tell whether the agent did real work or loaded a cache hit.

**Decision:**
Always pass `"(cached)"` as the Done suffix when a step returns early from the cache. Do not suppress the task line — the user needs to see that the step was evaluated and skipped, not omitted.

```go
chalk.Task(sub, "Generating embeddings")
if vec := chalk.Recover[[]float32](key, nil); vec != nil {
    chalk.Done("(cached)")    // ✓ visible, labelled skip
    return vec, nil
}
// ... real work ...
chalk.Commit(key, vec)
chalk.Done(fmt.Sprintf("(%d dims)", len(vec)))
```

For a task group where all items were cached, summarise at the parent level:

```go
allCached := true
for _, chunk := range chunks { /* ... */ }
if allCached {
    chalk.Done("(all cached)")
} else {
    chalk.Done(fmt.Sprintf("(%d new, %d cached)", newCount, cachedCount))
}
```

**Consequences:**
- The user immediately understands the speed difference between a cached and uncached run.
- The progress tree shape is identical for both runs, so the user can compare them visually.

---

## Quick-Reference: Output Patterns

```
TASK LABEL       gerund, title-cased          "Processing documents"
                 include artefact name         "Parsing chunk 3 of 10"

DONE SUFFIX      one metric, parenthetical     "(1 024 chunks)"
                 cache hit                     "(cached)"
                 nothing notable               (omit suffix entirely)

NOTE (Printf)    actionable or diagnostic      "⚠ field \"x\" ignored"
                 one note per issue            not a progress counter

ERROR (Fail)     what + why + what to do       "file.json: line 4: …"
                 wrapped context chain         "embed: api: rate limited"

SEPARATOR        once, end of run              ━━━ × 66 chars

NESTING          two levels maximum
                 level 0: pipeline phase
                 level 1: item or sub-phase
                 level 2: diagnostic detail only

TIMING RULE      < 1 s  → single task, no sub-tasks
                 1–5 s  → single task + Done suffix
                 5–30 s → sub-tasks or midpoint note
                 > 30 s → sub-tasks mandatory

ITERATION        ≤ 10   → one sub-task per item
                 ≤ 100  → one sub-task per item, suffix on notable only
                 ≤ 1 000 → batch sub-tasks (25/batch)
                 > 1 000 → parent task + interval Printf note
```
