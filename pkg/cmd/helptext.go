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
	ResourceAttempt       = "attempt"
)

// Short help (one line) for common commands. Use when the only difference is the resource name.
func ShortGet(resource string) string     { return "Get " + resource + " details" }
func ShortList(resource string) string    { return "List " + resource + "s" }
func ShortDelete(resource string) string  { return "Delete a " + resource }
func ShortDisable(resource string) string { return "Disable a " + resource }
func ShortEnable(resource string) string  { return "Enable a " + resource }
func ShortUpdate(resource string) string  { return "Update a " + resource + " by ID" }
func ShortCreate(resource string) string { return "Create a new " + resource }
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

// Beta help text for gateway/connection commands. Centralised so the feedback URL and wording stay consistent.
const BetaFeedbackSuffix = `

[BETA] This feature is in beta. Please share bugs and feedback via:
https://github.com/hookdeck/hookdeck-cli/issues`

// ShortBeta appends [BETA] to a short description for use in Cobra Short.
func ShortBeta(short string) string {
	return short + " [BETA]"
}

// LongBeta appends the beta feedback paragraph to a long description for use in Cobra Long.
func LongBeta(long string) string {
	return long + BetaFeedbackSuffix
}
