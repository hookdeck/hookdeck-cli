# Hookdeck CLI Planning Documents

## Connection Management - Production Ready ✅

**Status:** 98% complete and production-ready

See [`connection-management-status.md`](./connection-management/connection-management-status.md) for comprehensive documentation of the completed implementation.

**Key Achievements:**
- ✅ Full CRUD operations (create, list, get, upsert, delete)
- ✅ Complete lifecycle management (enable, disable, pause, unpause, archive, unarchive)
- ✅ Source authentication (96+ types) - [Commit 8acf8d3](https://github.com/hookdeck/hookdeck-cli/commit/8acf8d3)
- ✅ Destination authentication (HTTP, CLI, Mock API) - [Commit 8acf8d3](https://github.com/hookdeck/hookdeck-cli/commit/8acf8d3)
- ✅ All 5 rule types (retry, filter, transform, delay, deduplicate) - [Commit 8acf8d3](https://github.com/hookdeck/hookdeck-cli/commit/8acf8d3)
- ✅ Rate limiting configuration
- ✅ Idempotent upsert with dry-run support - [Commit 8ab6cac](https://github.com/hookdeck/hookdeck-cli/commit/8ab6cac)

**Optional Enhancements (Low Priority - 2% remaining):**
- Bulk operations (enable/disable/delete multiple connections)
- Connection count command
- Connection cloning

## Documentation and Transformation Updates ✅

**REFERENCE.md generation:**
- `REFERENCE.md` is now generated from Cobra command metadata via `go run ./tools/generate-reference`
- See `tools/generate-reference/main.go` and `REFERENCE.template.md`

**Transformation examples:**
- All transformation code examples updated from `module.exports = async (r) => r` to the correct Hookdeck format: `addHandler("transform", (request, context) => { return request; })`
- Applied in: pkg/cmd (create, run, upsert), README.md, REFERENCE.md (via regen), test/acceptance (helpers, transformation_test.go)
- Transformation run API response model aligned with OpenAPI `TransformationExecutorOutput` (uses `request` field for transformed payload)
- CLI adds default `content-type: application/json` when request headers are empty so the transformation engine executes successfully

**README rebalance:**
- Added Sources and destinations subsection (within Manage connections) with examples and link to [REFERENCE.md#sources](REFERENCE.md#sources) and [REFERENCE.md#destinations](REFERENCE.md#destinations)
- Added Transformations section with examples and link to [REFERENCE.md#transformations](REFERENCE.md#transformations)
- Added Requests, events, and attempts section with examples and links to [REFERENCE.md#requests](REFERENCE.md#requests), [REFERENCE.md#events](REFERENCE.md#events), [REFERENCE.md#attempts](REFERENCE.md#attempts)

## Active Planning Documents

- **[`connection-management-status.md`](./connection-management/connection-management-status.md)** - Current implementation status (98% complete)
- **[`resource-management-implementation.md`](./resource-management-implementation.md)** - Overall resource management plan

## Testing and sandbox

- **Always run tests** when implementing or changing code (`go test ./pkg/...`, and for CLI changes `go test ./test/acceptance/...`). Do not skip tests to avoid failures.
- If tests fail due to **TLS/certificate errors**, **network**, or **sandbox** (e.g. `x509`, `operation not permitted`), **prompt the user** and **re-run with elevated permissions** (e.g. `required_permissions: ["all"]`) so tests can pass.

## Development Guidelines

All CLI development follows the patterns documented in [`AGENTS.md`](../AGENTS.md):
- OpenAPI to CLI conversion rules
- Flag naming conventions
- Type-driven validation patterns
- Command structure standards
- **Ordered array configurations** - For API arrays with ordering (rules, steps, middleware)
- **Idempotent upsert pattern** - For declarative resource management with `--dry-run` support

Design specifications have been consolidated into `AGENTS.md` as general principles with connection management as concrete examples.
