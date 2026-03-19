---
name: compare-seatunnel-plugin-baseline
description: "Compare official SeaTunnel releases and propose connector/dependency baseline updates with human review checkpoints."
---

# Compare SeaTunnel Plugin Baseline

Use this skill when the user wants to compare a new official SeaTunnel version with the local plugin baseline template, detect connector and dependency changes, and generate a human-reviewable update proposal.

## Goal

Produce a review-first diff for:
- connector catalog changes
- `plugin-mapping.properties` changes
- connector POM dependency changes (**only treat `provided` as automatic diff input**)
- JDBC profile changes from `connector-jdbc/pom.xml` and dialect source additions
- candidate updates to local seed JSON / baseline rules

Do **not** silently overwrite the local baseline. Always stop for human review before applying.

## Primary sources

Only use official sources:
- `apache/seatunnel` Git tags
- `apache/seatunnel-website` versioned docs (when docs help explain dependencies)

Prefer tag source files over rendered docs.

## Inputs to confirm

Ask or infer:
- target version (required)
- baseline version (default: latest value in `reviewed_versions`)
- scope:
  - full connector catalog
  - JDBC profiles only
  - dependency diff only
  - all of the above

## Local files to inspect first

Read only what is needed:
- `internal/apps/plugin/seed/seatunnel-plugins.json`
- `.trellis/tasks/03-16-plugin-official-dependency-baseline/design.md`
- related plugin service code if target-dir or baseline rules matter

## Comparison workflow

1. Fetch connector directories for both versions from official tag source.
   - Compare `seatunnel-connectors-v2/**/pom.xml` module names
   - Compare `plugin-mapping.properties`
2. Build connector diff sets:
   - added
   - removed
   - renamed / remapped
   - unchanged
3. For each changed/new connector, inspect the module POM:
   - dependencies excluding `test`
   - **only elevate `provided` dependencies to automatic review candidates**
   - treat compile/runtime deps as auxiliary signals only, not as direct baseline-update evidence
4. For JDBC:
   - diff `seatunnel-connectors-v2/connector-jdbc/pom.xml`
   - inspect dialect factory additions/removals in source
   - map database profile -> artifacts -> version range
5. Classify each result into one of:
   - no extra dependency
   - companion connector only
   - isolated dependency under `plugins/<mapping>`
   - shared dependency under `lib/`
   - manual review required
6. Apply existing project rules while classifying:
   - built-in package jars are excluded from extra dependency recommendations
   - `file-base` / `file-base-hadoop` are hidden companion connectors
   - `http-base` remains visible but acts as companion for `http-*`
   - do not auto-promote ambiguous JDBC cases to defaults
   - **POM compile/runtime dependency changes do not auto-generate baseline changes**
7. Generate a **human review table** before any patch.

## Required output format

Always output these sections:

### 1. Summary
- target version
- baseline version
- connector diff counts
- dependency diff counts

### 2. Connector diff table
Columns:
- connector
- change type
- mapping change
- baseline impact
- review needed

### 3. Dependency/profile diff table
Columns:
- connector/profile
- artifact(s)
- old rule
- new official signal (`provided`, JDBC source change, or manual note)
- recommended action

### 4. Proposed JSON patch plan
Group into:
- add connector/profile
- remove connector/profile
- update versions
- update target_dir policy
- mark manual-review

### 5. Human confirmation questions
Ask concise batch questions such as:
- approve all exact version bumps?
- keep ambiguous connectors as manual-review?
- apply JDBC profile additions now?

## When the user approves changes

Only then:
- update the seed JSON / baseline files
- update design docs if rules changed
- keep the patch scoped to the approved rows

## Guardrails

- Do not treat local baseline as truth for connector catalog freshness.
- Do not infer external dependency delivery from docs alone if tag source contradicts it.
- Do not auto-apply new JDBC profiles without showing the table first.
- Do not mix “official signal” and “project override” in the same column.
- Do not promote compile/runtime dependency changes from POM into automatic baseline updates.
- If evidence is weak, classify as `manual_review`.
