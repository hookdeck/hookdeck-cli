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

## Active Planning Documents

- **[`connection-management-status.md`](./connection-management/connection-management-status.md)** - Current implementation status (98% complete)
- **[`resource-management-implementation.md`](./resource-management-implementation.md)** - Overall resource management plan

## Other Resources

- **[`localhost-quickstart.mdoc`](./localhost-quickstart.mdoc)** - Quick start guide for local development

## Development Guidelines

All CLI development follows the patterns documented in [`AGENTS.md`](../AGENTS.md):
- OpenAPI to CLI conversion rules
- Flag naming conventions
- Type-driven validation patterns
- Command structure standards
- **Ordered array configurations** - For API arrays with ordering (rules, steps, middleware)
- **Idempotent upsert pattern** - For declarative resource management with `--dry-run` support

Design specifications have been consolidated into `AGENTS.md` as general principles with connection management as concrete examples.