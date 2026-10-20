# ARL React Console Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the Vue console with a responsive React/TypeScript/Vite application and refactor the API around the new information architecture.

**Architecture:** Organization-centric read models are exposed by the existing Go modular monolith. React consumes a checked-in OpenAPI contract through a typed client; server data lives in TanStack Query and shareable filters live in the URL.

**Tech Stack:** Go 1.27, React 19, TypeScript, Vite 7, Tailwind CSS 4, shadcn/ui-style Radix components, TanStack Query/Table, React Router, React Hook Form, Zod, ECharts, Vitest, Testing Library, Playwright.

**Spec:** This plan and the user-approved design in the implementation request dated 2026-10-20.

## Global Constraints

- Chinese-only UI and light theme only.
- No Project domain model and no compatibility redirects or API aliases.
- Preserve cookie auth, refresh, CSRF, RBAC, masking, and server-side authorization.
- Use Node 22 for frontend installation and verification.
- Do not push, publish, or commit secrets/generated build artifacts.

## Tasks

- [ ] Task 1: Refactor HTTP API, add overview/search read models, unify catalog filters, update OpenAPI and Go tests.
- [ ] Task 2: Scaffold React/TypeScript/Vite foundation, shared design system, typed API client, auth/router/providers, and frontend tests.
- [ ] Task 3: Build overview, navigation shell, command palette, map/graph/inspector, and organization workflows.
- [ ] Task 4: Build assets, findings, tasks, automation, intelligence, admin, login, and password-change experiences.
- [ ] Task 5: Rewrite E2E/visual/accessibility coverage, update docs, run full verification, review, and commit.

## Review Focus

- Viewer/operator/admin authorization must be enforced server-side, not only hidden in navigation.
- Overview/search/map/graph must remain bounded and degrade cleanly when Neo4j, GeoIP, or ICP is unavailable.
- Mobile layouts must expose the same core triage workflows without tables overflowing.
- Response schemas must match the checked-in OpenAPI spec before TypeScript generation.
- Existing uncommitted React/OpenAPI work must be integrated without reverting unrelated files.
