// Package bridge runs a resource declared with an SDK v2 schema, and built and read with
// SDK v2 style functions over a *schema.ResourceData, inside a terraform-plugin-framework
// resource. It converts the SDK schema into the framework schema with the same wire shape
// (state written by the SDK implementation decodes unchanged), turns the framework plan
// and state into the ResourceData the build and read functions expect, and turns the
// ResourceData back into framework state, with the zero values the SDK writes for unset
// Optional attributes made null again.
//
// It exists so families with many type-specific schemas (the connectors) move to the
// framework without rewriting every build and read function. The SDK runtime is not
// involved: only its schema types and the ResourceData value container are used.
package bridge
