// Package framework holds the building blocks for resources written against
// terraform-plugin-framework: the shared attributes and model, the session lookup, the
// error handling built on internal/apierr, import by ID or identity from the same
// ImportSpec the SDK v2 importers use, and the state upgrader that migrates state written
// by an SDK v2 implementation. New resources use this package, never the SDK helpers.
package framework
