<!--
Sync Impact Report
- Version change: template-placeholder -> 1.0.0
- Modified principles:
	- Principle 1 placeholder -> I. Open Source Stewardship
	- Principle 2 placeholder -> II. Stable Public Contracts
	- Principle 3 placeholder -> III. Verification-First Changes (NON-NEGOTIABLE)
	- Principle 4 placeholder -> IV. Security and Supply-Chain Integrity
	- Principle 5 placeholder -> V. Simplicity, Operability, and Accountability
- Added sections:
	- Contribution and Decision Standards
	- Delivery and Release Governance
- Removed sections:
	- None
- Templates requiring updates:
	- ✅ reviewed (no content change required): .specify/templates/plan-template.md
	- ✅ reviewed (no content change required): .specify/templates/spec-template.md
	- ✅ reviewed (no content change required): .specify/templates/tasks-template.md
	- ⚠ pending (directory absent, nothing to update): .specify/templates/commands/*.md
	- ✅ reviewed (no content change required): README.md
	- ✅ reviewed (no content change required): docs/quickstart.mdx
- Follow-up TODOs:
	- None
-->

# Ollama Repository Constitution

## Core Principles

### I. Open Source Stewardship
All repository changes MUST preserve transparency, maintainability, and community
collaboration. Contributors MUST document intent for non-trivial changes, use
reviewable commits, and avoid introducing process or code that limits legitimate
external contribution.

Rationale: This repository is open source and must remain understandable,
auditable, and welcoming to sustained community maintenance.

### II. Stable Public Contracts
Changes to externally consumed behavior (CLI, APIs, file formats, model metadata,
or documented workflows) MUST be treated as contract changes. Contract changes
MUST include explicit compatibility notes and migration guidance before merge.

Rationale: Users depend on predictable interfaces; unmanaged breaking changes
damage trust and adoption.

### III. Verification-First Changes (NON-NEGOTIABLE)
Every change MUST include verification evidence proportionate to risk before
merge. At minimum, contributors MUST run affected tests or checks and record the
verification scope in pull requests. Code without executable verification MUST
include a justified limitation and a follow-up task.

Rationale: Regressions are cheaper to prevent than to triage after release.

### IV. Security and Supply-Chain Integrity
Contributors MUST prioritize secure defaults, least-privilege behavior, and
dependency hygiene. New dependencies, tooling, or build-path changes MUST be
justified, version-pinned where practical, and reviewed for licensing and
security impact.

Rationale: Infrastructure and model-serving systems are high-impact targets;
security discipline is a core reliability requirement.

### V. Simplicity, Operability, and Accountability
Designs MUST prefer the simplest solution that meets requirements. Changes MUST
preserve operability through clear logs, actionable errors, and diagnosable
runtime behavior. Ownership and rollback strategy MUST be clear for high-risk
changes.

Rationale: Operational clarity and constrained complexity improve reliability,
on-call response, and long-term contributor velocity.

## Contribution and Decision Standards

- Proposals and pull requests MUST state problem, scope, risks, and validation
	approach.
- Significant behavior changes MUST include explicit acceptance criteria and
	user-facing impact notes.
- Architectural or governance-impacting decisions MUST be documented in-repo in
	a durable, discoverable location.
- Reviewers MUST block changes that violate any Core Principle unless an
	approved exception is documented in the pull request with expiry or follow-up.

## Delivery and Release Governance

- Releases MUST include clear notes on behavior changes, deprecations, and
	known limitations.
- Breaking changes MUST include a migration path and timing guidance before
	release.
- Emergency fixes MAY use an expedited path, but MUST include retrospective
	verification and documentation updates immediately after stabilization.
- Release and maintenance branches MUST preserve traceability from source
	change to released artifact.

## Governance

This constitution is the highest-priority governance document for repository
engineering practices.

- Amendment Procedure: Amendments MUST be proposed in a pull request that
	includes rationale, affected principles/sections, and template impact
	analysis. Approval requires maintainer review and explicit acknowledgment of
	downstream updates.
- Versioning Policy: Constitution versions use Semantic Versioning.
	- MAJOR: Incompatible governance changes, principle removals, or
		redefinitions.
	- MINOR: New principle or materially expanded governance guidance.
	- PATCH: Clarifications, wording improvements, and non-semantic refinements.
- Compliance Review: Every pull request review MUST verify compliance with the
	Core Principles and applicable governance sections. Periodic audits SHOULD
	sample merged pull requests for adherence and identify corrective actions.

**Version**: 1.0.0 | **Ratified**: 2026-04-28 | **Last Amended**: 2026-04-28
