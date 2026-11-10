package cmd

// SuccessCheck is the checkmark character used in success messages across all commands.
const SuccessCheck = "✔"

// Resource names for shared help text (singular form for "a source", "a connection").
const (
	ResourceSource         = "source"
	ResourceConnection     = "connection"
	ResourceDestination    = "destination"
	ResourceTransformation = "transformation"
	ResourceEvent          = "event"
	ResourceRequest        = "request"
	ResourceAttempt        = "attempt"
	ResourceIssue          = "issue"

	// Outpost resources. Destination and attempt names are shared with the
	// Event Gateway constants above, but the Outpost resources they describe are
	// different, so the help text is composed per command rather than reused.
	ResourceTenant          = "tenant"
	ResourceTopic           = "topic"
	ResourceDestinationType = "destination type"
)

// Short help (one line) for common commands. Use when the only difference is the resource name.
func ShortGet(resource string) string     { return "Get " + resource + " details" }
func ShortList(resource string) string    { return "List " + resource + "s" }
func ShortDelete(resource string) string  { return "Delete a " + resource }
func ShortDisable(resource string) string { return "Disable a " + resource }
func ShortEnable(resource string) string  { return "Enable a " + resource }
func ShortUpdate(resource string) string  { return "Update a " + resource + " by ID" }
func ShortCreate(resource string) string  { return "Create a new " + resource }
func ShortUpsert(resource string) string  { return "Create or update a " + resource + " by name" }

// LongGetIntro returns the first paragraph for "get" commands: "Get detailed information about a specific {resource}.\n\nYou can specify either a {resource} ID or name."
// Callers append their own Examples block.
func LongGetIntro(resource string) string {
	return "Get detailed information about a specific " + resource + ".\n\nYou can specify either a " + resource + " ID or name."
}

// LongUpdateIntro returns the first sentence for "update" commands.
func LongUpdateIntro(resource string) string {
	return "Update an existing " + resource + " by its ID."
}

// LongDeleteIntro returns the first sentence for "delete" commands.
func LongDeleteIntro(resource string) string {
	return "Delete a " + resource + "."
}

// LongDisableIntro returns the first sentence for "disable" commands.
func LongDisableIntro(resource string) string {
	return "Disable an active " + resource + ". It will stop receiving new events until re-enabled."
}

// LongEnableIntro returns the first sentence for "enable" commands.
func LongEnableIntro(resource string) string {
	return "Enable a disabled " + resource + "."
}

// LongUpsertIntro returns the first sentence for "upsert" commands (create or update by name).
func LongUpsertIntro(resource string) string {
	return "Create a new " + resource + " or update an existing one by name (idempotent)."
}

// LongSourceAuthType explains how config.auth_type is filled in for source auth. configFlag is
// the flag that takes source config JSON ("--config" or "--source-config").
func LongSourceAuthType(configFlag string) string {
	return "Source types that offer a choice of verification method (WEBHOOK, HTTP and others) need " +
		"auth_type beside auth in the source config. When " + configFlag + " or the auth flags leave it " +
		"out, it is set from the auth fields: header_key and api_key -> API_KEY, username and password -> " +
		"BASIC_AUTH, webhook_secret_key with algorithm, encoding and header_key -> HMAC, and " +
		"webhook_secret_key alone -> STANDARD_WEBHOOKS (Standard Webhooks signatures, not HMAC). " +
		"Set auth_type in " + configFlag + " to choose the method yourself."
}
