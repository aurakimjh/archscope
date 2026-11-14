# ArchScope User Manual

This manual covers the Go engine and Wails native desktop application. The screen names and
procedures follow the current repository. It helps developers, operators, architects, and
database specialists explain **what was observed, what remains uncertain, and what evidence
to inspect next**.

Start with sections 1–4, then choose a screen for your symptom. Read the
[Analysis Workspace](#analysis_workspace) procedures before saving or comparing investigations.
The [Korean manual](../ko/USER_GUIDE.md) covers the same screens and workflows.

## Contents

- [1. What ArchScope helps you do](#purpose)
- [2. Getting started and common controls](#start)
- [3. Choosing evidence and an analyzer](#choose)
- [4. Reading any result](#reading)
- [5. The 12 analysis screens](#analyzers)
  - [Profiler](#profiler), [Diff](#diff), [Access log](#access_log), [GC log](#gc_log)
  - [JFR](#jfr), [Exception](#exception), [Thread dumps](#thread_dump), [MSA timeline](#msa_profile)
  - [Trace import](#trace_import), [HTTP capture](#http_capture), [Browser CPU](#browser_cpu), [Lighthouse audit](#browser_audit)
- [6. Analysis Workspace: building an investigation](#analysis_workspace)
- [7. The seven downstream workspace tools](#workspace-tools)
  - [Export Center](#export_center), [Report Diff](#report_diff), [Chart Studio](#chart_studio)
  - [Incident Timeline](#incident_timeline), [SLO / Golden Signals](#slo_golden_signals)
  - [Service Flow](#service_flow), [Evidence Board](#evidence_board)
- [8. Settings](#settings)
- [9. Guided investigations](#walkthroughs)
- [10. Troubleshooting, glossary, and completion checks](#troubleshooting)
- [Appendix: build, CLI, and further reading](#appendix)

<a id="purpose"></a>

## 1. What ArchScope helps you do

A slow service needs more than one kind of evidence. Access logs identify affected requests.
Profiles identify expensive execution or waiting paths. Thread dumps show where threads were
at particular instants. Traces connect service calls. Each answers a different question.

ArchScope analyzes files locally, collects results in a workspace, and helps you compare,
visualize, organize evidence, and prepare reports. A useful investigation proceeds as follows:

1. State the symptom and scope: for example, whether order lookup became slower after deployment.
2. Prepare logs, profiles, or dumps covering that target and interval.
3. Analyze the files and check parsing, coverage, and missing observations first.
4. Narrow the result to individual events, call paths, and original references.
5. Collect related results in the workspace and explicitly save a case.
6. Test hypotheses with another source and compare compatible before/after measurements.
7. Report observations, interpretations, uncertainty, and the next verification step separately.

A finding is a starting point for investigation. A long database call alone does not identify
whether the cause was a query plan, lock, connection acquisition, network, or database load.
Unrecorded events cannot be recovered simply by opening a file.

Check the [importer support matrix](IMPORTER_SUPPORT_MATRIX.md) for exact evidence support.
A language name does not guarantee every producer, version, or export layout is supported.
ArchScope does not provide a Java heap-dump (`.hprof`) analysis engine or automatic source
modification. General profile, system, and GPU imports read previously collected evidence.
Windows live HTTP capture is a separate, explicit collection feature.

<a id="start"></a>

## 2. Getting started and common controls

### First analysis

Launch the installed desktop app. For source builds, see the [appendix](#appendix) and
[native app instructions](NATIVE_APP.md). The archived Python/FastAPI application is not the
current execution path.

To learn the workflow with the repository sample:

1. Open **Access log** and select `examples/access-logs/sample-nginx-access.log`.
2. Select the nginx format and run analysis.
3. Check completion and parser diagnostics, then request counts, latency, status, and URL tables.
4. Open **Analysis Workspace** and find the generated result card.
5. Select that result in **Export Center** and save HTML or JSON.

Samples teach the interface; their findings are not measurements of your production system.

### Navigation and controls

Analysis tools read source files. Workspace tools reuse results. There are 12 analysis
screens, Analysis Workspace plus seven downstream tools, and Settings.

| Control | How to use it |
| --- | --- |
| File chooser/drop area | Select a local source supported by that analyzer. Multiple files require a screen supporting multiple inputs. |
| Recent files | Restore a path. A moved, deleted, or changed source is not restored as the old analysis. |
| Analyzer options | Set format, time scope, mode, and limits. Parsing/calculation changes require a new run. |
| Analyze/cancel/status | Distinguish completion, failure, and cancellation. An older visible result does not prove the latest run succeeded. |
| Summary cards | Read unit, population, and scope before drawing conclusions. |
| Search/sort/filter | Narrow the current view. Display filters and import options differ; not every filter recalculates denominators. |
| Help icons | Explain a control or section. Also inspect detailed engine warnings. |
| Source and timestamps | Distinguish collection time, result creation time, and workspace registration time. |

Several dedicated analyzers automatically register successful results in the workspace.
Selected MSA scopes and general evidence imports use an explicit **Add to Workspace** action.
Verify the card's type, source, and time after analysis.

<a id="choose"></a>

## 3. Choosing evidence and an analyzer

| Question or symptom | Evidence to prepare | Start here | Follow with |
| --- | --- | --- | --- |
| Which URLs are slow or failing? | Supported server/proxy access logs | Access log | Request trace/MSA; GC/profile for the same interval |
| High CPU or an expensive code path | CPU profile/collapsed stacks | Profiler | CPU metrics and a comparable baseline |
| Slow responses with low CPU | Wall-clock profile and consecutive dumps | Profiler / Thread dumps | Lock owner, connection pool, DB, downstream evidence |
| JVM pauses or memory oscillation | Supported GC log and JFR events | GC log / JFR | Request latency, allocation profile, memory metrics |
| Repeated exceptions | Exception logs/stack traces | Exception | Matching request/time in access logs and traces |
| Persistent stacks or suspected deadlock | Ordered dumps from the same process | Thread dumps | Lock ownership and wall-clock profile |
| Split an MSA request into calls/SQL | Jennifer transaction profile exports | MSA timeline | Callee profile, DB evidence, distributed trace |
| Cross-service latency/error propagation | Supported trace exports | Trace import | Service Flow, HTTP, related logs |
| Inspect client-observed HTTP | HAR or supported Windows capture session | HTTP capture | Server trace/access log, session comparison |
| Slow browser/Node JavaScript | Chrome Performance JSON / V8 .cpuprofile | Browser CPU | Lighthouse and HTTP |
| Loading quality/resource waste | Previously generated Lighthouse JSON | Lighthouse audit | Browser CPU/HTTP and a repeat under matching conditions |
| OS, metrics, serving, or GPU investigation | Supported originals and capture context | Workspace evidence import | Specialized detail views, projections, comparison |
| Did a change improve performance? | Compatible profiles or saved results | Diff / Report Diff / Workspace comparison | Repeated workload and coverage/quality checks |

Before analysis, record the symptom, target, collection interval/time zone, producer/version,
process identity and restarts, clock kind, sample period, loss, workload, deployment version,
and original file location. Remove secrets and private data from shared copies.

Leave unknown context unknown. File modification time is not collection time. Renaming a file
extension or concatenating incompatible exports does not make a supported input.
See [input assistance and manual collection](INPUT_ASSISTANCE.md).

<a id="reading"></a>

## 4. Reading any result

### Check parsing and scope first

After completion, inspect source files, processed counts, time scope, and diagnostics.
Unsupported lines, timestamp failures, limits, and partial results restrict the conclusion.
Analyzing five minutes of an hour does not establish that the whole hour was healthy.

Top N can limit ranked output, while max lines limits input processing. Some options also
select events. Read the relevant help and diagnostics; visible row counts are not necessarily
the full observation population.

### Read values with units and denominators

| State | Meaning | Next step |
| --- | --- | --- |
| Observed zero | A measurement recorded zero in its scope | Check unit and coverage |
| Empty / — / null / unavailable | Missing value or unmet calculation conditions | Inspect reason and source |
| Partial / truncated / limit reached | Incomplete processing or display | Record exclusions; reduce scope or prepare another input |
| Pending / working | Not yet determined | Wait for a final state |
| Unsupported / comparison refused | Unsupported format, feature, or conditions | Use a supported export or compatible new evidence |

Sample count is not elapsed time. A thousand samples are not automatically a thousand
milliseconds. Already weighted durations must not be multiplied by the sample period again.
A URL's error rate and the overall request error rate have different populations.
Some queries retain the selected-total denominator when filtering displayed rows.

p95 is a percentile of the observations under a declared calculation policy. It does not
mean 95% of requests each took that duration. Inspect tail latency as well as averages and
sample counts. Small populations make high percentiles hard to interpret. Averaging group
p95 values does not produce a combined p95. See [percentile policies](PERCENTILE_POLICIES.md).

### Separate observation, hypothesis, and action

“Order lookup p95 increased” is an observation. “Database lock waits caused it” is a
hypothesis. Check SQL timing and database wait/lock evidence before selecting a remedy.
Do not increase memory, thread counts, or connection pools solely because a warning appears.
After a change, repeat the collection under comparable conditions and check latency,
errors, throughput, and side effects.

### Reading the supplied originals

Use the repository's teaching files to connect source fields with an analysis question.
These examples explain sample content; they are not production incident verdicts.

| Sample | Source detail | Interpretation and next step |
| --- | --- | --- |
| [nginx access log](../../examples/access-logs/sample-nginx-access.log) | First GET has status 200 and duration 0.123 seconds; the 500 DELETE takes 1.500 seconds | Read 123ms and 1,500ms in this format. Inspect method/URL groups instead of treating DELETE and GET as the same request type. |
| [Java thread dump](../../examples/thread-dumps/sample-java-thread-dump.txt) | Request thread is BLOCKED in OrderService.find waiting for a monitor; housekeeper is TIMED_WAITING | Inspect lock ID and stack. This sample alone does not establish owner/deadlock; obtain further ownership and consecutive-dump evidence. Housekeeper waiting does not prove a broken pool. |
| [Wall profile](../../examples/profiler/sample-wall.collapsed) | Semicolon-separated frame paths end in a weight; one socketRead path has weight 13282 | Do not read this as 13,282ms without unit/period evidence. Investigate the read-wait path using request/downstream evidence. |

The current nginx sample produces six requests and two analyzer-counted errors.
This does not prescribe an operational policy of treating only 500 responses as errors.
Inspect the separate 404 and 500 status groups and apply service context.

<a id="analyzers"></a>

## 5. Analysis screens

<a id="profiler"></a>

### 5.1 Profiler

**Purpose:** identify the call paths dominating the selected measurement. CPU profiles
describe execution cost; wall-clock profiles help investigate elapsed time including waits.
Collapsed inputs offer wall/cpu/lock profile kinds. For allocation and other measurements,
check supported originals and workspace metric selection; retain producer measurement semantics.

1. Select supported collapsed stacks, Jennifer exports, or flamegraph input and the correct format.
2. Match profile kind and value unit to the source. Enter an interval only when known.
   An explicit elapsed duration must have collection evidence.
3. Check Top N and memory guards, then analyze.
4. Read summary/diagnostics, top stacks, flamegraph, and detailed paths.
5. Click frames to zoom; use include/exclude or regex filters. Keeping parents and re-rooting
   around a match present different context.
6. If needed, set a timeline base method representing the transaction boundary and rerun.

A wide flamegraph frame carries more weight. Horizontal position is not event chronology.
Self is the frame's own cost; inclusive includes children. Adding parent and child values
double counts. High inclusive with low self calls for inspecting callees.
A wide database wait in a wall profile is not evidence of high CPU in that frame.

Execution breakdown and timeline classifications explain observed evidence using rules.
Unordered aggregate stacks do not reconstruct the exact sequence of a real individual request.
Estimated time remains an estimate, and a category label is not a cause determination.

**Next:** correlate CPU paths with CPU metrics; correlate wait paths with consecutive dumps,
SQL, and downstream evidence. For exact original py-spy/Speedscope/pprof selections, use
[workspace profile queries](#profile-query).

<a id="diff"></a>

### 5.2 Diff: comparing profiles

**Purpose:** identify paths that increased, decreased, appeared, or disappeared.

1. Select the pre-change baseline and post-change target.
2. Check kind, metric/unit, workload, sampling, and collection duration.
3. Decide whether normalization is appropriate when totals differ, then compare.
4. Inspect changed paths in both original profiles.

Normalization adjusts totals to help compare relative composition; it does not establish an
absolute response-time or CPU improvement. A smaller share can still cost more if the whole
run became longer. Do not use color alone as an improvement verdict.

This screen compares profile inputs. Use [Report Diff](#report_diff) for saved general results,
or a [case baseline](#case-baseline) for repeatable comparison with original-source verification.

<a id="access_log"></a>

### 5.3 Access log

**Purpose:** locate user impact by URL, status, and time before investigating internal causes.

1. Select a supported server log and format. Check whether latency is actually present.
2. Set start/end time and max lines as needed, then analyze.
3. Inspect volume, latency distribution, and response status.
4. Review URL counts, latency, and errors together; distinguish rare long requests from broad impact.
5. Find latency/error intervals to investigate in traces, GC, and profiles.

Averages can hide long tails. Read p95/p99 and maximum alongside population size.
4xx responses do not all indicate server faults; 5xx may originate in an application,
proxy, or upstream service. The measurement boundary determines included waiting/transfer time.
Missing latency is not zero milliseconds.

**Next:** locate explicit request IDs or a matching interval in traces/MSA, then investigate
SQL, downstream, CPU, and GC. Record time zones, rotation gaps, and per-URL sample counts.

<a id="gc_log"></a>

### 5.4 GC log

**Purpose:** investigate whether JVM collection and pauses relate to latency or throughput loss.

1. Select supported logs; configure strict parsing, line limits, and Top N as needed.
2. Check collector, heap settings, and coverage in JVM information.
3. Locate long pauses and repeated patterns in the pause timeline.
4. Inspect heap before/after, available allocation/promotion signals, types/causes, and events.
5. Compare request latency over the same interval and retain related results.

A sawtooth heap can reflect ordinary allocation and reclamation. Check whether the
post-collection baseline rises and whether pressure coincides with user impact.
GC throughput is not service request throughput. A Full GC or long pause alone does not
prove a memory leak. Short or incomplete logs do not establish a trend.

**Next:** inspect JFR allocation/profile evidence, use a separate specialist tool when object
retention analysis is required, or native-memory events for native allocations.
Record collector/JVM/workload conditions and repeat after a change.

<a id="jfr"></a>

### 5.5 JFR

**Purpose:** investigate CPU, wall-clock, allocation, lock, GC, exception, or I/O activity
within a recording.

1. Choose supported JFR JSON. For binary recordings, preconvert with a compatible JDK or use
   the explicit installed-JDK conversion flow in [input assistance](INPUT_ASSISTANCE.md).
2. Select mode, time window, thread state, and Top N.
3. Read scope/contract and diagnostics to establish which events and fields were recorded.
4. Inspect summary, flamegraph, event breakdown, and individual events.
5. When available, select a wall-clock heatmap bucket to narrow the start and check the window before rerunning.

An event disabled in recording settings is missing evidence, not evidence of no problem.
Event duration and CPU sample weight cannot be added as a single time total.

**Native Memory:** allocation/free matching identifies candidates remaining unfreed near the
recording end. Leak-only excludes matched pairs; tail exclusion reduces noise from recent
allocations. Distinguish bytes from event counts and inspect top-site stacks.
Missing events and frees outside the recording limit conclusions: a candidate is not a
confirmed leak. Distinguish observed zero, pending processing, failure, and unrecorded events.
This feature does not replace Java heap-dump analysis.

<a id="exception"></a>

### 5.6 Exception

**Purpose:** group repeated errors to locate important types and recurring stack locations.

1. Select supported exception logs/stack traces and appropriate options.
2. Read type and signature groups.
3. Open events for message, stack, nested causes, and original context.
4. Narrow by request, service, or error pattern and preserve important observations.

Multiple layers can log the same exception; occurrence count need not equal failed requests.
A signature groups similar stack shapes, not necessarily one business cause.
A parsed root-cause exception is distinct from the final system incident cause.

**Next:** check first occurrence, deployment/configuration changes, request IDs, and downstream
errors. Review messages for private data before including them in a report.

<a id="thread_dump"></a>

### 5.7 Thread dumps

**Purpose:** inspect execution locations and waiting relationships during hangs, exhaustion,
or contention. A dump is a snapshot; consecutive dumps provide evidence of persistence.

1. Choose Single for one instant, Multi for persistence, or Locks for supported lock metadata.
2. Select files in collection order from the same process; do not mix restarts or processes.
3. Override format only when automatic detection is incorrect.
4. In Multi, check persistence threshold; also review any provided correlation context. The threshold counts consecutive
   dumps, not seconds of observed duration.
5. Inspect states, signatures, individual threads, lock ownership/waits, and deadlock cycles.

WAITING/TIMED_WAITING can be normal idle-worker states. RUNNABLE in one dump does not prove
sustained CPU consumption. Repeated stacks require valid identity and collection intervals;
another source must distinguish repeated useful work from a stall.
Do not infer ownership where lock metadata is absent.

**Next:** inspect the owner as well as waiters, connection pools, DB waits, and downstream
timeouts. See [multi-language threads](MULTI_LANGUAGE_THREADS.md) for format and identity limits.

<a id="msa_profile"></a>

### 5.8 MSA timeline

**Purpose:** split Jennifer-exported requests into service, SQL, method, connection, and
related timing evidence to direct the next investigation.

1. Parse related profile files together. Check file errors, completeness, and missing profiles.
2. Choose a single real GUID or an average signature grouping similar transaction shapes.
3. Narrow to a TXID or application URL when needed.
4. Wait for the selected scope to finish, then inspect timeline, breakdown, SQL, and external calls.
5. Save JSON/HTML or add that selected result to Workspace. Disabled actions during invalid or
   pending selection prevent treating an old scope as the new selection.

| View | Interpretation |
| --- | --- |
| Single GUID | Observed request structure and elapsed values; check missing callees and incomplete profiles |
| Average signature | Check included samples/candidates and exclusions; not one real request's exact sequence |
| SQL | Read counts, totals, averages, and long calls; does not prove a plan or lock cause |
| External_Call / API | Callee time differs from caller-observed elapsed; nested/parallel intervals are not simply additive |
| Network Prep | Wrapper residual after excluding internal external-call time; not pure wire latency |
| Service network time | Matching caller/callee timing differences; inspect matching and measurement quality |
| Method residual | METHOD time remaining after identified child work; not a generic slow-method ranking or pure CPU time |
| Unprofiled external call | Caller observed, matching callee profile missing; does not establish failure or absence of the call |

Custom method rules/cards classify known patterns. Check rule consistency when comparing.
Residual totals and composition cards may differ due to scope, overlap handling, and display
limits. See [MSA dispatch](MSA_SERVLET_DISPATCH.md).

**Next:** investigate long SQL in DB evidence, external calls in callee profiles/traces,
and residual methods in CPU/wall profiles or dumps. State the selected scope in reports.

<a id="trace_import"></a>

### 5.9 Trace import

**Purpose:** inspect spans, service dependencies, latency, and error propagation.

1. Select a supported export, format, and Top N.
2. Inspect service/dependency summaries and locate a relevant trace.
3. Check parents, service, duration, and errors in trace/span tables.
4. Read critical-path semantics and duration availability.
5. Combine results in Service Flow and Incident Timeline.

Summing parallel span durations can exceed request elapsed time.
An observed span-chain estimate is not a complete distributed execution guarantee.
Missing parents, sampling, absent services, and clock differences limit reconstruction.
Matching times alone do not make two log entries the same request.

**Next:** follow explicit IDs, services, and endpoints into access logs, HTTP, or DB evidence.
Distinguish where a caller first reported an error from where a downstream failure occurred.

<a id="http_capture"></a>

### 5.10 HTTP capture

**Purpose:** inspect client-observed HTTP requests, responses, status, timing, and available
headers/body evidence. This boundary differs from server-side SQL or CPU measurement.

#### HAR import

1. Select supported HAR 1.2; check private data and body inclusion in the original.
2. Read summary, fidelity, capture mode, and diagnostics.
3. Use search, filters, sort, and grouping to narrow method, host, path, status, or latency.
4. Open transaction details for URL/query, headers, timing, and body availability/reasons.
5. If using stars/comments, complete the relevant save/export and reopen to verify them.

In **Save / export**, choose a format and scope before **Save…**.
Use ArchScope session for session preservation or supported HAR/text/HTML formats for sharing.
Distinguish **All stored exchanges** from **Current displayed rows** and record the scope.
Use **Open session** for subsequent investigation and check annotations and omitted fields.

Import redaction is not an exhaustive detector of business secrets.
Retained raster pixels are separate from text redaction; review image content before sharing.
Omitted bodies are not empty bodies; unrecorded fields cannot be recovered.
See [HTTP capture](SYSTEM_HTTP_CAPTURE.md) for feature and protocol limits.

#### Windows live capture

Use supported Windows environments; macOS file analysis does not provide Windows collection.

1. Read first-use proxy/CA guidance and choose the capture target.
2. Follow temporary CA installation when HTTPS interception is required.
3. Start capture and configure the intended test client to use the displayed loopback proxy.
4. Generate only intended traffic; inspect status, attribution, drops, and persistence.
5. Stop, confirm finalization, then analyze the stored session.
6. Restore any proxy/trust settings separately configured in external clients.

Bodies are omitted by default. A per-session opt-in appears only when supported preconditions
are met. Windows trust and JVM/NSS trust can differ. Unattributed metadata retention is an
explicit choice and defaults to drop.

The latest 500 live rows are not the whole persisted session. For longer runs, check the
finalized store, counters, and drops. After display operations such as Clear, check the
current session guidance to distinguish visible and retained data.
Confirmed attribution describes connection endpoint/PID evidence, not complete traffic coverage.
Pending, unsupported, passthrough, and not-captured states are not successful semantic capture.
Proxy bypass, certificate pinning, and protocols such as QUIC can restrict visibility.

#### Comparison, correlation, and request composition

Assign baseline/target in workspace HTTP comparison. Correlation accepts compatible evidence
and reports alignment eligibility/refusal. Temporal overlap is not causality.

For request composition, review method, destination, headers, and body before explicitly
sending. Check environment and authority for requests causing changes. Opening a saved request
and transmitting it are separate actions. Changed client/authentication/network conditions
can produce different behavior from the original observation.

<a id="browser_cpu"></a>

### 5.11 Browser CPU

**Purpose:** inspect sampled JavaScript CPU paths and observed runs from Chrome Performance
traces or V8 `.cpuprofile`, including supported Node/CDP exports.

1. Select supported JSON or gzip input and analyze.
2. Check input type, coverage, and partial warnings.
3. Inspect wide flamegraph paths and self/inclusive top frames.
4. Inspect sampled-run rows and stacks.
5. Follow with HTTP and Lighthouse evidence for the same interaction.

Respect weight units. Do not equate V8 weighted durations with JVM sample counts or multiply
already weighted values by an interval. Self is direct cost; inclusive includes children.
This is CPU sample analysis, not full network/layout/paint attribution or a complete
DevTools replacement. Sampled runs do not enumerate every browser long task.
See [Chrome/V8 CPU evidence](CHROME_DEVTOOLS_CPUPROFILE.md).

<a id="browser_audit"></a>

### 5.12 Lighthouse audit

**Purpose:** review loading metrics, audits, and resource opportunities in an existing report.

1. Select local Lighthouse JSON; this flow does not automatically run an audit against a URL.
2. Inspect original score and report conditions.
3. Review Web Vitals-related metrics, findings, and audits.
4. Inspect network/resource tables and connect them with CPU/HTTP evidence.
5. Use available JSON/HTML export to retain results.

A single lab score is not the performance of all real users or proof of an operational SLO.
Device, network, throttling, cache, and version affect measurements.
Repeat under matching conditions and distinguish browser execution changes from server changes.

<a id="analysis_workspace"></a>

## 6. Analysis Workspace: building an investigation

### 6.1 Why use it?

An investigation often needs access logs, GC, traces, and profiles together.
Analysis Workspace collects results, selects them for downstream tools, and supports saved
cases, queries, comparisons, evidence cards, and reporting.

| Concept | Meaning | Use |
| --- | --- | --- |
| Original source | Collected/exported log, profile, or dump | Reparse or inspect original evidence |
| Analysis result | Calculation from particular bytes and options | Reuse summary, tables, and observations |
| Workspace card | Result registered in the current application | Select for other tools |
| Saved case | Explicitly saved results, references, and investigation context | Continue after restart |
| Pinned baseline | Immutable binding to a saved result, source, and conditions | Repeat comparison against one fixed reference |
| Evidence card | Selected observation/row/visual and provenance | Communicate supporting facts |

A visible workspace card is not an explicit case save. Save needed results as a case or JSON.
Attaching an external original reference does not automatically back up the entire source.
Moving or changing it can break search, original-coordinate lookup, and reimport verification.

### 6.2 Collect, select, and organize results

1. Verify results registered by dedicated analyzers or import evidence below.
2. Activate the intended card.
3. Check type, source, and time; verify the same result in downstream selectors.
4. Save important results before removing unwanted cards.
5. Confirm preservation before clearing the workspace.

The current workspace retains the latest 80 entries. Older cards can leave the list during
repeated analyses; explicitly save important results as cases/JSON first.

Some tools use one selected result. Incident Timeline and Service Flow use multiple workspace
entries. Activating one card does not exclude the others.
Check for mixed incidents, repeated imports, and before/after results when interpreting combined views.

### 6.3 Import and explore evidence

1. Enter a local path or use **Browse** in **Import and explore evidence**.
2. Use format detection/input assistance to inspect candidates and limits. Select an importer
   explicitly when required.
3. For source-assistance panels, inspect the original, choose profile/metric and capture
   conditions, and save a new normalized model first. Check the applied importer/options.
4. Review JSON import options and explicit partial-result allowance, then **Analyze**.
   Original files, normalized models, and saved AnalysisResult JSON are different inputs.
5. Resolve any requested importer selection; inspect completed/partial/failed/cancelled status.
6. Select available summary/table/timeline/comparison providers. Check totals, limits,
   availability reasons, and previous/next pages.
7. **Add to Workspace** and **Save result JSON** or explicitly save a case.

Use **Open saved result** for an existing AnalysisResult JSON.
Opening it does not reanalyze its original. Older versions may offer only generic read-only views.
For stale result references, reopen saved JSON or a case snapshot.

Exact backend results preserve large integers and decimals. Keep original JSON rather than
using shortened screen values as the only record. Unsupported capabilities and versions
provide refusal/disabled reasons that should be resolved before reuse.

<a id="profile-query"></a>

### 6.4 Canonical profile drilldown and evidence

The main questions are which frames have direct cost, who calls them, where cost goes next,
and which observations support the result.

1. Inspect a supported original py-spy/Speedscope/pprof source and explicitly select profiles
   and metric. The first worker/thread need not represent the whole capture.
2. Open **Canonical profile drilldown and evidence**.
3. Use Top self/inclusive sorting to separate direct and descendant cost.
4. Expand/focus paths in tree/flamegraph.
5. Use frame IDs for callers/callees.
6. Read evidence/original references and apply ordered conjunctive frame-ID filters if needed.
7. Use **Add selection as new result** for downstream tools; the original result remains.

Literal search and display filters retain the selected-total denominator. Read total,
metric/unit, and row shares together. Evidence time windows use declared start timestamps
and half-open `[from,to)` semantics; they do not clip intervals or redistribute cost.
Untimed observations are not assigned invented times.

A high-inclusive/low-self method may be an entry point to an expensive child.
Inspect callees and observed paths before selecting an optimization.
See [original profile import](PROFILE_SOURCE_IMPORT.md) and
[case evidence queries](CASE_EVIDENCE_QUERIES.md).

### 6.5 Additional evidence inside Workspace

These are source/detail panels within evidence import, not additional top-level routes.
Consult the linked documentation for actual producer/version coverage.

| Evidence | Available workflow | Interpretation limits |
| --- | --- | --- |
| [Metric history](METRIC_HISTORY.md) | Inspect, select series, query intervals, create selection, export CSV | Distinguish gauge/counter/rate; inspect resets, gaps, missing values |
| [Apple Silicon/Ollama](APPLE_SILICON_EVIDENCE.md) | Import supported manual captures and inspect memory/serving observations | Actual swap/pressure observations are required; missing is not healthy zero |
| [perf / limited BPF](SYSTEM_EVIDENCE.md) | Events, intervals, histograms, supported profile projections | On/off-CPU, run queue, and I/O differ; off-CPU does not prove lock contention |
| [Inference serving](INFERENCE_EVIDENCE.md) | Device/serving signals, metrics, and linkage | Device utilization is not request latency; inspect unlinked series and availability |
| [LLM trace](LLM_TRACE_EVIDENCE.md) | Trace/span and token/request evidence | Distinguish TTFT, total latency, request and aggregate throughput populations |
| [Runtime](RUNTIME_EVIDENCE.md) | Supported runtime evidence, metrics, and source detail | Check process/lifetime/clock; arbitrary text is not a supported dump |
| [Benchmark/crash](EXECUTION_EVIDENCE.md) | Supported summaries/JTL/hs_err measurements and comparisons | Success/all-request populations differ; crash logs are not heap/core analysis |
| [CPU/GPU execution](GPU_EXECUTION_EVIDENCE.md) | Supported PyTorch/Nsight operators, kernels, copies, links, overlap | CPU evidence is not CUDA evidence; summary CSV does not reconstruct intervals |
| [DB/OS snapshots](SYSTEM_DB_SNAPSHOTS.md) | Supported manual snapshots and metric intervals | Distinguish a point observation from a trend; check adapter support |

Metric-history windows select interval **end** timestamps in `[from_ns,to_ns)`.
Intervals starting outside the window remain whole. This differs from profile evidence
start-time selection. Do not combine distinct units, resources, attributes, or boundaries.
The first counter has no invented previous zero. Histograms and precomputed percentiles
do not authorize an invented combined p95.

### 6.6 Save, reopen, and search cases

For a quick snapshot, open **Saved cases → Save selected result**.
Reopen with **Open case → case → saved result**. The action requires an available backend result.

For original linkage and search, use **Case search and reusable baselines**:

1. Create a case or list/select an existing one.
2. **Load current revision**.
3. **Attach original reference** as a read-only external source.
4. Select the source and **Save current result with source**.
5. Choose source/result scope and search field, then search text, IDs, or digest.
6. Open original coordinates to inspect supporting content; **Open exact snapshot** restores the saved result.

Search covers indexed material with prefix, snippet, and budget limits.
No hit does not prove absence from the entire source.
Typed time searches require one result and its declared clock/unit; inspect untimed counts.
Arbitrary log timestamps or text coordinates are not invented.

Reload the case after external changes or cancelling a mutation: it may already have committed.
Do not bypass stale revisions or digest mismatches by editing hashes.
Cache cleanup removes unowned temporary results separately; save/export needed results first.

<a id="case-baseline"></a>

### 6.7 Pinning and comparing a verified baseline

**Purpose:** repeatedly compare against a fixed result while rechecking original bytes,
selection, metric, and compatible conditions.

1. Link a supported exact original-profile result and its source in a case.
2. Select the saved result/source and **Reimport and pin new baseline**.
3. Link and save the target with its original.
4. Select the immutable baseline and target result/source.
5. Supply complete entity mappings and reasons only when appropriate.
6. **Verify originals and compare**; inspect eligibility, scope, units, and deltas.

A baseline does not follow later analysis automatically. Create new results/pins without
overwriting originals. Missing/changed sources or different metrics, selections, clocks,
and conditions can prevent comparison.

Declared identity is capture context supplied by its author.
Verified-by-reimport establishes agreement between the referenced bytes/options and model;
it does not authenticate the collection environment or producer.
A snapshot-only saved result is not reimport verification.

Resolve refusals through source preservation, correct selections, or comparable recollection.
Relabeling units or inventing clock equality is not a remedy.
See [case queries and baseline rules](CASE_EVIDENCE_QUERIES.md).

### 6.8 HTTP comparison/correlation and AI interpretation

HTTP entries expose comparison and correlation panels when applicable.
Assign baseline and target separately; inspect candidates and alignment reasons.
Aligned overlapping evidence is not automatic proof of causality.

AI interpretation is optional and local. When a payload is present, inspect provider, model,
prompt version, disabled state, finding count, and gate. Review evidence references before
using reportable findings in Evidence Board or Report Pack.
Keep AI explanations distinct from deterministic results. Analysis/export remain usable
without an available model. See [AI interpretation](AI_INTERPRETATION.md).

<a id="workspace-tools"></a>

## 7. Workspace tools

<a id="export_center"></a>

### 7.1 Export Center

**Purpose:** preserve a selected result for sharing or later investigation.

1. Choose the correct workspace entry.
2. Confirm type, source, and summary preview.
3. Select an available HTML, PowerPoint, CSV, CSV-directory, or JSON format.
4. Choose a destination, run export, then open and check the saved artifact.

Use original JSON for exact result reuse, supported HTML/PowerPoint for reading, CSV for
tabular inspection, visual/table actions for slides, and Report Pack for multiple pieces
of evidence and context.

Not every result supports every format. Backend exact entries disable PowerPoint and
CSV-directory options; HTML/CSV also require support for that result type.
CSV need not contain every original event/field. Keep JSON alongside a readable report
when needed. Review URLs, headers, source content, and annotations for private information.

<a id="report_diff"></a>

### 7.2 Report Diff

**Purpose:** inspect summary metric and finding changes in two saved AnalysisResult JSON files.

1. Select Before and After JSON, with useful labels.
2. Run comparison and inspect types, metric deltas, and finding changes.
3. Check units, source scope, counts, and options in the originals.

A difference does not prove equivalent experimental conditions.
A vanished warning could reflect improvement, missing data, reduced scope, or a policy change.
Use case baselines when original verification and a reusable fixed reference are needed.

<a id="chart_studio"></a>

### 7.3 Chart Studio

**Purpose:** create charts from compatible workspace results for reuse.

1. Select a result and an available chart template.
2. Choose Canvas/SVG and inspect the preview.
3. Check title, legend, axes, units, and scope, then save PNG, SVG, or CSV as supported.

No template may mean the result has no compatible chart. Renderer selection changes
presentation, not source statistics.

In supported result details, **Report image and table** offers columns, title, dimensions,
font, rows per image, annotation, and **Render pinned view**.
Then copy images, copy the complete selected table, save images/provenance, or retain on
Evidence Board. A visual is a pinned snapshot: regenerate after changing its source/scope.
Retain all pages and provenance for split tables. Native rendering/clipboard requires a
desktop session. See [visual artifacts](VISUAL_ARTIFACTS.md).

<a id="incident_timeline"></a>

### 7.4 Incident Timeline

**Purpose:** organize events from multiple results into a time-ordered incident narrative.

1. Collect results for the same incident.
2. Check event/group/source counts and partial/omitted status.
3. Filter by text, analyzer, severity, or group.
4. Read narrative, time, source, correlation, and evidence references.
5. Add relevant events to Evidence Board.

Missing or incompatible timestamps limit ordering. Coincident GC warnings and request errors
are correlation candidates, not proof of causality. Record clock/time-zone and coverage limits.
Display filtering does not delete source events.

<a id="slo_golden_signals"></a>

### 7.5 SLO / Golden Signals

**Purpose:** inspect latency, traffic, errors, saturation, and available SLIs against targets.

1. Collect related results and inspect signal inventory/source coverage.
2. Check SLI values, units, scopes, and observation windows.
3. Review violations, actuals, thresholds, affected scopes, and available budget information.
4. Filter by signal kind, severity, or search.
5. Preserve important violations and return to supporting requests/events.

No violations does not mean every SLO passed: check unevaluated or absent targets.
Do not mix request populations or use one short experiment's p95 as a monthly SLO.
Report Pack recalculates derived measurements from current workspace sources.
Preserving valid saved target configuration is distinct from reusing historical violations.

<a id="service_flow"></a>

### 7.6 Service Flow

**Purpose:** combine trace/MSA evidence into caller→callee edges for latency/error review.

1. Prepare related trace/MSA results.
2. Inspect edge, call, error, and unmatched summaries.
3. Read caller/callee counts, average/max latency, network gap, and sources.
4. Filter by text/severity and add findings/edges as evidence.
5. Export Mermaid or JSON when needed.

The diagram describes imported evidence, not necessarily the entire current architecture.
Unmatched means no matching callee evidence, not a confirmed failed call.
Network gap is not necessarily pure wire latency; inspect measurement and matching quality.
Check sources to avoid treating duplicate requests across captures as real traffic growth.

<a id="evidence_board"></a>

### 7.7 Evidence Board

**Purpose:** curate observations and provenance supporting a report.

1. Add evidence from workspace cards, timeline/SLO/flow rows, and supported visuals/tables.
2. Check title, summary, severity, analyzer, and source references.
3. Remove duplication and confirm investigation scope.
4. Export card HTML/JSON or Report Pack HTML/ZIP.
5. Open the artifact and check units, visuals, provenance, and omitted content.

Prefer “SQL latency increased in this scope; these rows support it; locking remains
unverified” to an unsupported “the database is the cause.”
A summary card does not replace full results or originals. Save those separately when needed.
Confirm preservation before clearing the board. AI cards require evidence and a valid gate.

<a id="settings"></a>

## 8. Settings

Configure language, light/dark/system theme, default sample interval, Top N, profile kind,
recent files, and application information.

Save defaults, then inspect how a new analyzer initializes its options.
Clearing recent paths is distinct from deleting source logs.
Record version/build information with support requests.

Changing language does not translate imported messages or frame names.
Changing defaults does not recompute existing results. An interval default is not valid for
every input; actual declared units and collection conditions take precedence.

<a id="walkthroughs"></a>

## 9. Guided investigations

These are fictional teaching scenarios, not measured performance results or automatic thresholds.

### 9.1 Order lookup became slower after deployment

1. Ask whether latency changed under comparable request/data conditions.
2. Analyze before/after access logs with matching interval length and URL scope; check count,
   errors, p95, and affected times.
3. Locate explicit IDs in MSA/trace and split SQL, external calls, and method work.
   Without IDs, record the limitation of temporal correlation.
4. Follow long SQL into DB plan/wait/lock evidence; external calls into callee evidence;
   residual methods into profiles; coincident pauses into GC/JFR.
5. Save related workspace results with originals in a case. Keep timed observations in
   Timeline and supporting rows plus uncertainty on Evidence Board.
6. Recollect after the change, preserving workload/version/options. Compare profiles or a
   verified baseline, then check request latency and errors as well.
7. Report observation, reference, possible explanation, remaining checks, and post-change validation.

### 9.2 Responses hang without high CPU

1. Prepare consecutive same-process dumps with recorded intervals and a wall-clock profile.
2. Find persistent stacks in Multi and supported ownership/waits in Locks.
3. Separate idle workers from active request waits rather than classifying WAITING as failure.
4. Follow connection waits into pool/DB state and external waits into downstream/timeouts.
5. Save dump ordering, intervals, identity, and key stacks in a case.
6. After a remedy, check latency, errors, and throughput as well as the waiting pattern.

### 9.3 Browser interaction is slow despite fast server responses

1. Inspect HTTP/HAR for the same interaction and establish its measurement boundary.
2. Find high-self JavaScript paths and long sampled runs in Browser CPU.
3. Inspect Lighthouse audits/resources and relevant scripts.
4. Obtain more DevTools evidence for unobserved rendering/network phases instead of inventing attribution.
5. Repeat with matching device/cache/network conditions and preserve visuals/provenance.

### 9.4 LLM first response or generation is slow

1. Distinguish TTFT from total latency.
2. Import supported LLM traces, serving evidence, and metric history.
3. Check whether queue/prefill/decode are actually recorded; do not invent their durations.
4. Compare workload, concurrency, model/token conditions, and contemporaneous device evidence.
5. Add supported CPU/GPU execution data if available; utilization alone does not identify the bottleneck.
6. Separate successful-request distributions from started/completed/error populations and report missing signals.

<a id="troubleshooting"></a>

## 10. Troubleshooting, glossary, and completion checks

| Problem | Check in this order |
| --- | --- |
| Empty result | Supported producer/format, original content, diagnostics, time/filter scope, event collection settings |
| File/model/result too large | Identify the exceeded budget, reduce collection/selection, create a new result |
| JFR conversion fails | Compatible installed JDK/absolute path, input/output budgets/new destination, preconversion |
| Comparison disabled/refused | Metric/unit, selection, clock/entity, capture/quality conditions, source/version |
| Original lookup fails | File existence/change, current revision, source linkage/digest, supported coordinates |
| Old result lacks detail | Preserve generic read-only/JSON; explicitly reimport a supported original if needed |
| Stale result reference | Reopen saved JSON/case snapshot and select it again |
| No HTTP traffic | OS/session, client proxy, CA/truststore, protocol/bypass, attribution/drop |
| Export disabled | Result type and supported format; use JSON or supported report/table/visual |
| Empty chart/timeline | Compatible result, workspace registration, timestamps/units, filter/provider selection |
| AI unavailable | Continue deterministic analysis; inspect local model/provider, disabled reason, and gate |

Avoid cutting arbitrary bytes out of structured stacks/events. Prefer producer-supported
time-window or export selection. Normalized results can exceed budgets before the original
does; for example, case results have a 64 MiB limit. Inspect actual size and limit in the error.

### Glossary

| Term | Meaning |
| --- | --- |
| Evidence | Observations/calculations and their sources, distinct from unverified hypotheses |
| Finding | Analyzer observation/warning, not a final cause determination |
| Profile / metric | Selected observations and their measurement/unit |
| Self / inclusive | Frame's own weight / weight including descendants |
| Signature | Grouping identity for similar stacks or transaction shapes |
| Baseline / target | Reference / compared result; order determines delta direction |
| Capture / provenance | Collection context / source, tool, conversion, and selection history |
| Coverage / availability | Observation/processing scope / whether a value is available and why |
| Counter / gauge / rate | Cumulative measurement / point value / change per time |
| Clock / window | Time reference / selected range; relative and wall clocks are not automatically aligned |

### Before declaring the investigation complete

- Record symptom, target, interval/time zone, comparison conditions, and original storage.
- Check partial processing, loss, parser warnings, units, and denominators.
- Follow summaries down to events/stacks and original evidence.
- Separate facts, hypotheses, and remaining checks for the next owner.
- Save needed results as cases/JSON and verify reopening.
- Retain scope, units, provenance, and private-data review for report visuals/tables.
- Recheck comparable latency, errors, throughput, and side effects after changes.

<a id="appendix"></a>

## Appendix: build, CLI, and further reading

See [native application](NATIVE_APP.md) for builds and the
[importer support matrix](IMPORTER_SUPPORT_MATRIX.md) for exact evidence support.

### Source build and packaging

```bash
cd apps/engine-native
go test ./...
go build ./cmd/archscope-engine ./cmd/archscope-app

cd cmd/archscope-app/frontend
npm ci
npm run build
```

For desktop packaging:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
cd apps/engine-native/cmd/archscope-app
task package
```

### CLI examples

```bash
cd apps/engine-native

go run ./cmd/archscope-engine access-log analyze \
  --in ../../examples/access-logs/sample-nginx-access.log \
  --format nginx \
  --out access.json

go run ./cmd/archscope-engine profiler analyze-collapsed \
  --in ../../examples/profiler/sample-wall.collapsed \
  --out profiler.json

go run ./cmd/archscope-engine thread-dump analyze \
  --in ../../examples/thread-dumps/sample-java-thread-dump.txt \
  --out thread.json

go run ./cmd/archscope-engine trace import \
  --in ../../examples/traces/sample-otlp-traces.jsonl \
  --format auto \
  --out trace.json

go run ./cmd/archscope-engine database-log analyze \
  --in ../../examples/database/sample-postgres.log \
  --format postgres-text \
  --out database.json

go run ./cmd/archscope-engine broker-log analyze \
  --in ../../examples/broker/sample-broker.log \
  --format auto \
  --out broker.json

# Chrome Performance trace or V8 .cpuprofile (Node --cpu-prof, CDP)
go run ./cmd/archscope-engine profile import \
  --in ./trace.json.gz \
  --format auto \
  --out browser-profile.json

# Local Lighthouse report (scores are preserved, URLs are redacted)
go run ./cmd/archscope-engine browser import \
  --in ./lighthouse-report.json \
  --format lighthouse-json \
  --out browser-audit.json

# Redacted HAR import (dialect auto-detection, bounded entry cap)
go run ./cmd/archscope-engine http-capture analyze \
  --in ./session.har \
  --out http-capture.json

go run ./cmd/archscope-engine api-contract analyze \
  --openapi ../../examples/api-contract/openapi-orders.json \
  --access-result ../../examples/api-contract/access-result.json \
  --asyncapi ../../examples/api-contract/asyncapi-orders.json \
  --broker-result ../../examples/api-contract/broker-result.json \
  --out contract.json

go run ./cmd/archscope-engine stitch analyze \
  --in ../../examples/stitching/access-result.json \
  --in ../../examples/stitching/trace-result.json \
  --in ../../examples/stitching/database-result.json \
  --time-window-seconds 60 \
  --out stitched.json

go run ./cmd/archscope-engine architecture-docs draft \
  --in contract.json --in stitched.json \
  --out architecture-docs.json

go run ./cmd/archscope-engine report html \
  --in architecture-docs.json \
  --out architecture-docs.html
```

Run `go run ./cmd/archscope-engine --help` for the full command list. The
current supported evidence families are summarized in
[importer support matrix](IMPORTER_SUPPORT_MATRIX.md).


### Save an MSA selection from the CLI

Use the same GUID/TXID or signature/application scope as the UI. Obtain GUID/signature
values from an unscoped analysis. Omit TXID/application to select the whole GUID/signature.

```sh
archscope-engine jennifer-profile analyze --in profiles.txt --guid GUID --txid TXID --out scope.json
archscope-engine jennifer-profile report-html --in profiles.txt --guid GUID --txid TXID --out scope.html
archscope-engine jennifer-profile analyze --in profiles.txt --signature HASH --application /orders --out average-scope.json
```

### Further reading

- [Input assistance](INPUT_ASSISTANCE.md): discovery, manual kits, explicit JFR conversion.
- [Original profiles](PROFILE_SOURCE_IMPORT.md), [case queries/baselines](CASE_EVIDENCE_QUERIES.md): exact selection, source references, comparison.
- [Metric history](METRIC_HISTORY.md), [system evidence](SYSTEM_EVIDENCE.md): intervals, rates, OS evidence.
- [Visual artifacts](VISUAL_ARTIFACTS.md): rendering, clipboard, and preservation.
- [HTTP capture](SYSTEM_HTTP_CAPTURE.md), [browser CPU](CHROME_DEVTOOLS_CPUPROFILE.md): actual support and limits.
- [AI interpretation](AI_INTERPRETATION.md), [offline packs](OFFLINE_PACKS.md): optional features and setup.
