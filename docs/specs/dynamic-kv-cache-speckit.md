# Dynamic Context Capacity SpecKit (KV Cache Growth)

## 1. Purpose
This specification defines a portable, implementation-agnostic design for dynamic context capacity in model serving.

Primary objective:
- Keep generation progressing under context pressure by growing available cache capacity and only reloading when necessary.

Design center:
- System-capacity behavior across scheduler, runner, and cache backends.
- Not a literal copy of the current branch implementation details.

## 2. Core Model (Normative)
This section is the product intent and should drive implementation choices.

### 2.1 Terminology
1. Logical history:
The token timeline for a sequence, including input prefix/prompt tokens and newly generated tokens.
2. Context capacity (C):
How many token positions the runtime can hold for a sequence before growth/shift/stop handling is required.
3. Window size (W):
How much recent history certain attention layers are allowed to see (for sliding-window style layers).
4. Retained memory (M):
How much history is physically retained for that cache strategy. M may be equal to or greater than W.

### 2.2 Required interpretation
1. Logical history is the context substrate.
2. Prompt/prefix tokens and generated tokens both consume context positions.
3. Window is a moving subset/policy over logical history, not the whole history.
4. Increasing C and changing W are different controls and must be treated separately.

## 3. System Intent: Capacity Lifecycle (Normative)
Dynamic capacity is a system contract, not a single cache-type feature.

### 3.1 Scheduler contract
1. Scheduler loads a runner with an initial capacity plan.
2. That load establishes a baseline runtime envelope (placement, memory plan, and initial context allocation).

### 3.2 Runner contract
Runner owns live capacity management while serving requests.

Runner tracks:
1. C_loaded: planned capacity at load time.
2. C_allocated: currently allocated cache capacity.
3. C_required: capacity needed to continue current sequence execution safely.

### 3.3 Pressure handling contract
When C_required > C_allocated:
1. Attempt in-process capacity growth for the active cache backend.
2. If in-process growth is not possible in the current layout, trigger reload/reacquire path with updated capacity hints.
3. Resume continuation using already emitted output prefix when retry policy allows.
4. If growth/reload cannot satisfy capacity, apply engine-native limit behavior (same behavior as existing non-dynamic overflow policy in target engine).

## 4. Cache-Agnostic Requirement (Normative)
This spec is not causal-only by design.

1. Any cache strategy used by supported models (causal, sliding-window, chunked, hybrid) must have a valid capacity growth story.
2. A cache strategy may implement growth differently, but the scheduler-runner contract remains the same.
3. If a backend cannot grow in place, it must expose a deterministic fallback contract to scheduler/reload logic.
4. Supporting a cache strategy without growth is allowed only if the spec explicitly marks it as phased and defines fallback behavior.

Important distinction:
1. W may remain fixed while C grows.
2. That is valid and expected.
3. Growth increases continuation headroom; W still controls visibility rules for relevant layers.

## 5. Request Context Policy (Normative)
### 5.1 Client num_ctx handling
Client-provided num_ctx is a hint, not an immediate hard reconfiguration trigger for an already loaded compatible runner.

Policy for this spec:
1. Do not force reload simply because request num_ctx differs from current loaded context.
2. Continue on current runner when compatible.
3. Grow capacity on demand as actual sequence pressure appears.

Rationale:
1. Requesting a higher num_ctx does not guarantee the sequence will consume it.
2. Capacity should scale with real demand, not preemptive over-allocation.

### 5.2 Feature gating
1. Dynamic behavior must be feature-flagged.
2. Default state is disabled.
3. Environment-driven enablement is required.

## 6. Failure and Retry Semantics (Normative)
1. Growth failure must be surfaced with structured capacity metadata (from/to/max or equivalent).
2. Runner must remain healthy after recoverable growth/memory failures.
3. Retry behavior:
Default one retry attempt per request.
Retry count should be configurable by policy.
4. If growth and retry both fail, follow existing engine overflow/limit semantics for the target fork (do not invent a new behavior silently).

## 7. Observability and Performance Intent (Normative)
1. `ps` and `ps --verbose` must report capacity usage state accurately enough for operators.
2. Telemetry must not block or degrade inference critical paths.
3. If observability detail conflicts with throughput/latency, preserve inference performance first.
4. Stats collection must be bounded, best-effort, and timeout-safe.

## 8. Separation of Intent vs Decisions vs Caveats
Use this section to keep implementation work aligned.

### 8.1 Specification intent (must keep)
1. Dynamic context growth is a system-capacity feature.
2. Window policy is separate from capacity policy.
3. Scheduler-runner contract governs grow, reload, and continuation.

### 8.2 Implementation decisions (project-level, changeable)
1. Exact growth step sizing and clamping strategy.
2. Retry budget and backoff behavior.
3. Internal hint plumbing across scheduler and runner.
4. Metric field names and CLI formatting.

### 8.3 Current code caveats (non-normative reference only)
1. The branch that inspired this spec currently gates dynamic growth to specific cache paths.
2. Resume coverage may be conservative in transformed output pipelines.
3. Some decisions in that branch are risk/scope choices, not architectural constraints.

These caveats must not be mistaken for long-term product intent.

## 9. Portable Interface Expectations
A target fork should map these conceptual interfaces, regardless of file layout:
1. Cache backend interface:
init, grow, capacity-report, and deterministic failure metadata.
2. Runner interface:
pressure detection, growth invocation, retryable failure surfacing.
3. Scheduler interface:
reload with capacity/placement hints and idle-eviction policy hooks.
4. Stream interface:
prefix-preserving continuation for retries.
5. Observability interface:
non-blocking capacity stats for process listing.

## 10. Acceptance Criteria
1. Prompt plus generated tokens are accounted as one logical history sequence.
2. Window behavior is documented and tested as a subset policy over history.
3. Capacity growth can be exercised without tying the design to one cache type.
4. At least one non-causal cache strategy has a defined growth or deterministic fallback contract.
5. Runner survives growth-related memory failures without process crash.
6. Retry/resume preserves emitted-prefix continuity when enabled.
7. `ps --verbose` provides useful capacity telemetry without measurable inference-path regression in representative load tests.

## 11. Implementation Notes for This Repository (Informative)
These notes are references, not requirements for other forks.
1. Current branch introduced useful ideas:
dynamic capacity metadata, growth-triggered retry, and low-overhead live reporting.
2. Reuse those ideas, but keep product semantics above branch-specific constraints.
3. Prioritize scheduler-runner capacity orchestration over cache-type-specific branching when designing the target implementation.