package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// destinationConfigFlags holds destination config flags for create/upsert/update.
// Used by destination create, upsert, update. They are an alternative to
// --config/--config-file, not an overlay on it: each input describes the whole
// config, so naming both is refused by rejectConfigJSONWithIndividualFlags.
type destinationConfigFlags struct {
	URL                     string
	CliPath                 string
	AuthMethod              string
	BearerToken             string
	BasicAuthUser           string
	BasicAuthPass           string
	APIKey                  string
	APIKeyHeader            string
	APIKeyTo                string
	CustomSignatureSecret   string
	CustomSignatureKey      string
	RateLimit               int
	RateLimitPeriod         string
	DeliveryGroupKey        string
	DeliveryGroupRate       int
	DeliveryGroupRatePeriod string
	DeliveryGroupOverrides  string
	PathForwardingDisabled  *bool
	HTTPMethod              string
}

func addDestinationDeliveryPolicyFlags(cmd *cobra.Command, flags *destinationConfigFlags) {
	cmd.Flags().IntVar(&flags.RateLimit, "rate-limit", 0, "Rate limit (requests per period)")
	cmd.Flags().StringVar(&flags.RateLimitPeriod, "rate-limit-period", "", "Rate limit period (second, minute, hour, concurrent)")
	cmd.Flags().StringVar(&flags.DeliveryGroupKey, "delivery-group-key", "", "Payload field path used to group deliveries (for example body.customer_id)")
	cmd.Flags().IntVar(&flags.DeliveryGroupRate, "delivery-group-rate", 0, "Default maximum delivery rate for each delivery group")
	cmd.Flags().StringVar(&flags.DeliveryGroupRatePeriod, "delivery-group-rate-period", "", "Delivery group rate period (second, minute, hour)")
	cmd.Flags().StringVar(&flags.DeliveryGroupOverrides, "delivery-group-overrides", "", "JSON object of group-specific delivery rate overrides")
}

func addConnectionDestinationDeliveryPolicyFlags(cmd *cobra.Command, flags *connectionCreateCmd) {
	cmd.Flags().IntVar(&flags.DestinationRateLimit, "destination-rate-limit", 0, "Rate limit for destination (requests per period)")
	cmd.Flags().StringVar(&flags.DestinationRateLimitPeriod, "destination-rate-limit-period", "", "Rate limit period (second, minute, hour, concurrent)")
	cmd.Flags().StringVar(&flags.DestinationDeliveryGroupKey, "destination-delivery-group-key", "", "Payload field path used to group deliveries (for example body.customer_id)")
	cmd.Flags().IntVar(&flags.DestinationDeliveryGroupRate, "destination-delivery-group-rate", 0, "Default maximum delivery rate for each delivery group")
	cmd.Flags().StringVar(&flags.DestinationDeliveryGroupRatePeriod, "destination-delivery-group-rate-period", "", "Delivery group rate period (second, minute, hour)")
	cmd.Flags().StringVar(&flags.DestinationDeliveryGroupOverrides, "destination-delivery-group-overrides", "", "JSON object of group-specific delivery rate overrides")
}

func (f *destinationConfigFlags) validateDeliveryPolicyFlags(flagPrefix string) error {
	_, err := buildDeliveryPolicy(
		f.RateLimit,
		f.RateLimitPeriod,
		f.DeliveryGroupKey,
		f.DeliveryGroupRate,
		f.DeliveryGroupRatePeriod,
		f.DeliveryGroupOverrides,
		flagPrefix,
	)
	return err
}

// hasAnyDestinationConfig returns true if any individual destination config flag is set.
func (f *destinationConfigFlags) hasAnyDestinationConfig() bool {
	if f == nil {
		return false
	}
	return f.URL != "" || f.CliPath != "" || f.AuthMethod != "" ||
		f.BearerToken != "" || f.BasicAuthUser != "" || f.BasicAuthPass != "" ||
		f.APIKey != "" || f.APIKeyHeader != "" || f.CustomSignatureSecret != "" || f.CustomSignatureKey != "" ||
		f.RateLimit > 0 || f.RateLimitPeriod != "" || f.DeliveryGroupKey != "" ||
		f.DeliveryGroupRate > 0 || f.DeliveryGroupRatePeriod != "" || f.DeliveryGroupOverrides != "" ||
		f.PathForwardingDisabled != nil || f.HTTPMethod != ""
}

func buildDeliveryPolicy(rate int, period, groupKey string, groupRate int, groupRatePeriod, overridesJSON, flagPrefix string) (map[string]interface{}, error) {
	policy := make(map[string]interface{})
	// A negative rate is something the caller typed, so it has to be rejected
	// rather than treated as absent. Testing only `rate > 0` let `--rate-limit -5`
	// fall through both guards and be dropped, and the command then succeeded
	// having quietly ignored the value.
	if rate < 0 {
		return nil, fmt.Errorf("--%srate-limit must be a positive integer", flagPrefix)
	}
	if period != "" && rate == 0 {
		return nil, fmt.Errorf("--%srate-limit must be a positive integer when rate limiting is configured", flagPrefix)
	}
	if rate > 0 {
		if period == "" {
			return nil, fmt.Errorf("--%srate-limit-period is required when --%srate-limit is set", flagPrefix, flagPrefix)
		}
		policy["rate"] = rate
		policy["period"] = period
	}

	// Same again for the group rate: a negative value must count as configured,
	// or it is silently discarded instead of refused.
	hasGroups := groupKey != "" || groupRate != 0 || groupRatePeriod != "" || overridesJSON != ""
	if !hasGroups {
		return policy, nil
	}
	groupFlagPrefix := "--" + flagPrefix + "delivery-group-"
	if groupKey == "" {
		return nil, fmt.Errorf("%skey is required when delivery groups are configured", groupFlagPrefix)
	}
	if groupRate <= 0 {
		return nil, fmt.Errorf("%srate must be a positive integer when delivery groups are configured", groupFlagPrefix)
	}
	if groupRatePeriod == "" {
		return nil, fmt.Errorf("%srate-period is required when delivery groups are configured", groupFlagPrefix)
	}

	groups := map[string]interface{}{
		"key":         groupKey,
		"rate":        groupRate,
		"rate_period": groupRatePeriod,
	}
	if overridesJSON != "" {
		var overrides map[string]interface{}
		if err := json.Unmarshal([]byte(overridesJSON), &overrides); err != nil {
			return nil, fmt.Errorf("%soverrides must be a valid JSON object: %w", groupFlagPrefix, err)
		}
		if overrides == nil {
			return nil, fmt.Errorf("%soverrides must be a valid JSON object", groupFlagPrefix)
		}
		groups["overrides"] = overrides
	}
	policy["groups"] = groups
	return policy, nil
}

// cliPathFromFlags returns the --cli-path value only when the user actually
// supplied it. The flag carries a "/" default on create, so comparing against ""
// would let an unset flag overwrite a path given via --config.
func cliPathFromFlags(cmd *cobra.Command, cliPath string) string {
	if cmd != nil && !cmd.Flags().Changed("cli-path") {
		return ""
	}
	return cliPath
}

// applyCLIPath sets the path for a CLI destination. An explicit --cli-path wins;
// otherwise a path already supplied via --config is left alone. withDefault adds
// the "/" default, which only create does — upsert leaves the field absent so the
// stored value survives a partial update.
func applyCLIPath(config map[string]interface{}, cliPath string, withDefault bool) {
	if cliPath != "" {
		config["path"] = cliPath
		return
	}
	if _, ok := config["path"]; ok {
		return
	}
	if withDefault {
		config["path"] = "/"
	}
}

// nestedMap walks a chain of map keys, returning false if any level is missing
// or is not itself a map.
func nestedMap(m map[string]interface{}, keys ...string) (map[string]interface{}, bool) {
	cur := m
	for _, k := range keys {
		if cur == nil {
			return nil, false
		}
		next, ok := cur[k].(map[string]interface{})
		if !ok {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

// deliveryGroupsNeedOverrides reports whether config sets delivery_policy.groups
// without supplying overrides, which is the case that would destroy stored ones.
func deliveryGroupsNeedOverrides(config map[string]interface{}) bool {
	groups, ok := nestedMap(config, "delivery_policy", "groups")
	if !ok {
		return false
	}
	_, given := groups["overrides"]
	return !given
}

// preserveDeliveryGroupOverrides carries delivery_policy.groups.overrides
// forward from the stored config when the caller did not supply its own.
//
// The API merges delivery_policy one level deep but replaces groups wholesale,
// so sending a groups object without overrides silently destroys them. Since
// the CLI requires --delivery-group-key and --delivery-group-rate-period
// whenever --delivery-group-rate is given, "just bump the rate" always sends a
// full groups object, and was always the command that lost the overrides.
func preserveDeliveryGroupOverrides(config, existingConfig map[string]interface{}) {
	if !deliveryGroupsNeedOverrides(config) {
		return
	}
	existing, ok := nestedMap(existingConfig, "delivery_policy", "groups")
	if !ok {
		return
	}
	if overrides, ok := existing["overrides"]; ok {
		groups, _ := nestedMap(config, "delivery_policy", "groups")
		groups["overrides"] = overrides
	}
}

// applyStoredDestinationConfig fills in what an upsert PUT needs from the stored
// destination. Two distinct cases share the lookup:
//
//   - a partial update (only --description, say) sends no config at all, and the
//     API requires one on PUT, so the stored config is carried forward;
//   - a delivery_policy.groups object sent without overrides would replace the
//     stored groups wholesale and take the overrides with it (#393), so the
//     stored overrides are carried forward.
//
// The two differ in how a failed lookup has to be treated. The first can carry
// on: the worst outcome is the API rejecting a config-less PUT, which is visible.
// The second cannot: continuing sends the bare groups object and destroys the
// overrides this is here to protect, and the PUT succeeds, so nothing reports it.
func applyStoredDestinationConfig(name string, req *hookdeck.DestinationCreateRequest, lookup func() (*hookdeck.Destination, error)) error {
	if len(req.Config) > 0 {
		return preserveStoredDeliveryGroupOverrides(name, req.Config, lookup)
	}

	// Partial update: the API requires a config on PUT, so the stored one is
	// carried forward. A failed lookup is tolerable here — the worst outcome is
	// the API rejecting a config-less PUT, which the user sees.
	existing, err := lookup()
	if err != nil || existing == nil || existing.Config == nil {
		return nil
	}
	req.Config = existing.Config
	if req.Type == "" {
		req.Type = existing.Type
	}
	return nil
}

// preserveStoredDeliveryGroupOverrides carries the stored
// delivery_policy.groups.overrides into config when config sets groups without
// them, and refuses to proceed if it cannot read them.
//
// The API replaces groups wholesale, so sending a bare groups object destroys
// the stored overrides (#393) — and the PUT succeeds, so nothing reports it.
// That makes a failed lookup unrecoverable: carrying on would do the exact
// damage this is here to prevent. Every command that can send a groups object
// goes through here, so update, upsert and connection upsert cannot drift.
func preserveStoredDeliveryGroupOverrides(name string, config map[string]interface{}, lookup func() (*hookdeck.Destination, error)) error {
	if !deliveryGroupsNeedOverrides(config) {
		return nil
	}
	existing, err := lookup()
	if err != nil {
		return fmt.Errorf("failed to look up destination %q to preserve its delivery group overrides; refusing to send a delivery group that would replace them: %w", name, err)
	}
	if existing == nil || existing.Config == nil {
		return nil
	}
	preserveDeliveryGroupOverrides(config, existing.Config)
	return nil
}

// hasAnyDeliveryPolicyFlag reports whether a rate-limit or delivery-group flag
// was given. It decides whether resolving the stored destination type is worth
// an extra API call.
func (f *destinationConfigFlags) hasAnyDeliveryPolicyFlag() bool {
	if f == nil {
		return false
	}
	return f.RateLimit != 0 || f.RateLimitPeriod != "" || f.DeliveryGroupKey != "" ||
		f.DeliveryGroupRate != 0 || f.DeliveryGroupRatePeriod != "" || f.DeliveryGroupOverrides != ""
}

// typeSpecificDestinationFlags are the flags whose meaning depends on the
// destination type: buildDestinationConfigFromIndividualFlags reads each one
// only under the type whose config actually has that field. A value given for
// any other type is dropped from the request and the command still reports
// success, which is how --url went missing on a typeless update (#406).
var typeSpecificDestinationFlags = []struct {
	name      string
	appliesTo string
	given     func(*destinationConfigFlags) bool
}{
	{"url", "HTTP", func(f *destinationConfigFlags) bool { return f.URL != "" }},
	{"http-method", "HTTP", func(f *destinationConfigFlags) bool { return f.HTTPMethod != "" }},
	{"path-forwarding-disabled", "HTTP", func(f *destinationConfigFlags) bool { return f.PathForwardingDisabled != nil }},
	{"cli-path", "CLI", func(f *destinationConfigFlags) bool { return f.CliPath != "" }},
}

// destinationIndividualConfigFlags are the flags that set a field inside the
// destination config, which is exactly what --config and --config-file supply
// wholesale.
var destinationIndividualConfigFlags = []string{
	"url", "cli-path", "http-method", "path-forwarding-disabled",
	"auth-method", "bearer-token", "basic-auth-user", "basic-auth-pass",
	"api-key", "api-key-header", "api-key-to",
	"custom-signature-secret", "custom-signature-key",
	"rate-limit", "rate-limit-period",
	"delivery-group-key", "delivery-group-rate", "delivery-group-rate-period",
	"delivery-group-overrides",
}

// rejectConfigJSONWithIndividualFlags refuses --config or --config-file next to
// a flag that sets one of the same fields.
//
// The two ways of describing a config disagreed with each other and the three
// commands disagreed about how. --config was documented as winning, and did on
// `update`; `create` and `upsert` then overlaid --url and --cli-path back on
// top of it, but `upsert` only reached that overlay when --type was passed,
// because resolveDestinationType returns early on the --config path. So
// `upsert --config '{"url":"https://old"}' --url https://new` exited 0 having
// sent the old URL, while the same flags with --type HTTP sent the new one, and
// `update` sent the old one either way (the #406 shape, in a corner).
//
// Refusing the combination is what fixes all of that at once. The alternative -
// making the individual flag win everywhere - only reaches the two fields the
// overlays happen to cover: --auth-method, --http-method, --rate-limit and the
// delivery-group flags would still be dropped in silence under --config, and
// merging them in raises questions (what happens to delivery_policy.groups?)
// that nobody has asked for. Either input describes the whole config, so asking
// for one is unambiguous and asking for both never was.
func rejectConfigJSONWithIndividualFlags(cmd *cobra.Command, configStr, configFile string) error {
	if configStr == "" && configFile == "" {
		return nil
	}
	jsonFlag := "--config"
	if configStr == "" {
		jsonFlag = "--config-file"
	}
	for _, name := range destinationIndividualConfigFlags {
		flag := cmd.Flags().Lookup(name)
		// Changed, not the value: --api-key-to and --cli-path carry defaults,
		// and a default the user never typed is not a conflict.
		if flag == nil || !flag.Changed {
			continue
		}
		return fmt.Errorf("--%s cannot be combined with %s: %s supplies the whole config, so put the field in the JSON or drop %s",
			name, jsonFlag, jsonFlag, jsonFlag)
	}
	return nil
}

// hasAnyTypeSpecificFlag reports whether a flag was given that only one
// destination type has a field for.
func (f *destinationConfigFlags) hasAnyTypeSpecificFlag() bool {
	if f == nil {
		return false
	}
	for _, flag := range typeSpecificDestinationFlags {
		if flag.given(f) {
			return true
		}
	}
	return false
}

// needsResolvedType reports whether any given flag is one whose handling depends
// on the destination type, and so whether resolving the stored type is worth an
// API call. Both kinds count: the delivery-policy flags, which are refused on a
// CLI destination, and the type-specific flags, which are only read under their
// own type.
func (f *destinationConfigFlags) needsResolvedType() bool {
	return f.hasAnyDeliveryPolicyFlag() || f.hasAnyTypeSpecificFlag()
}

// destinationTypeIsKnown reports whether this is a type the CLI builds config
// for. An unknown non-empty type is reported by the config builder itself,
// which names the supported set.
func destinationTypeIsKnown(destType string) bool {
	switch strings.ToUpper(destType) {
	case "HTTP", "CLI", "MOCK_API":
		return true
	}
	return false
}

// rejectTypeSpecificFlagsForOtherTypes refuses a type-specific flag that the
// type in hand has no field for, including the case where the type is not known
// at all. Silence was the old behaviour in both directions: the value was left
// out of the request body and the command exited 0 (#406).
func rejectTypeSpecificFlagsForOtherTypes(destType string, f *destinationConfigFlags) error {
	if f == nil {
		return nil
	}
	t := strings.ToUpper(destType)
	if t != "" && !destinationTypeIsKnown(t) {
		return nil
	}
	for _, flag := range typeSpecificDestinationFlags {
		if !flag.given(f) {
			continue
		}
		if t == "" {
			// Only reachable when there is no stored destination to resolve the
			// type from — an upsert that is really a create.
			return fmt.Errorf("--%s cannot be applied without a destination type: pass --type (HTTP, CLI, MOCK_API)", flag.name)
		}
		if t != flag.appliesTo {
			return fmt.Errorf("--%s applies to %s destinations, and this destination is %s; the API would drop it", flag.name, flag.appliesTo, t)
		}
	}
	return nil
}

// resolveDestinationType resolves the destination type that the config flags
// have to be interpreted against.
//
// `destination update` and `destination upsert` normally omit --type, and two
// separate things went wrong because of it. The delivery-policy guard compared
// against "" and passed, so rate-limit and delivery-group flags reached a stored
// CLI destination where the API accepts the request and discards the policy
// (#392). And config building switches on the type, so --url and --cli-path
// were never copied into the request body at all, and the command still exited
// 0 (#406).
//
// Resolving the stored type is preferred over refusing the command, because the
// type the user omitted is already knowable and every one of these commands is
// addressing a destination the API can name. It is deliberately not conditioned
// on which kind of flag was given: resolving only for the policy flags would
// have made --url start working when a rate-limit flag happened to be present
// too, which is a worse contract than failing uniformly.
//
// lookup returns the stored destination, or (nil, nil) when there is none —
// an upsert that is really a create, where a typeless request is the API's to
// reject. It is only called when the answer can change the outcome.
func resolveDestinationType(declaredType string, usesConfigJSON bool, flags *destinationConfigFlags, lookup func() (*hookdeck.Destination, error)) (string, error) {
	if declaredType != "" || usesConfigJSON || !flags.needsResolvedType() {
		return declaredType, nil
	}
	existing, err := lookup()
	if err != nil {
		return "", fmt.Errorf("failed to look up the destination to resolve the type its config flags apply to: %w", err)
	}
	if existing == nil {
		return declaredType, nil
	}
	return existing.Type, nil
}

// fetchDestinationByName returns the stored destination with this exact name, or
// (nil, nil) when none exists. The list endpoint filters by name, but returns a
// summary, so the full record is fetched for its config.
func fetchDestinationByName(ctx context.Context, client *hookdeck.Client, name string) (*hookdeck.Destination, error) {
	listResp, err := client.ListDestinations(ctx, map[string]string{"name": name})
	if err != nil {
		return nil, err
	}
	if listResp == nil || len(listResp.Models) == 0 {
		return nil, nil
	}
	return client.GetDestination(ctx, listResp.Models[0].ID, nil)
}

// rejectDeliveryPolicyForCLI refuses delivery-policy flags on a CLI destination.
// CLI destinations carry no delivery_policy in the API schema: the request is
// accepted and the policy discarded, so without this the flags look applied and
// never take effect. destType must be the resolved type, not the raw --type flag
// — see resolveDestinationType.
func rejectDeliveryPolicyForCLI(destType string, policy map[string]interface{}, flagPrefix string) error {
	if len(policy) == 0 || strings.ToUpper(destType) != "CLI" {
		return nil
	}
	return fmt.Errorf("--%srate-limit and --%sdelivery-group-* are not supported for CLI destinations", flagPrefix, flagPrefix)
}

// rejectDeliveryPolicyInConfigForCLI applies the same guard to an already-built
// config. update and upsert build the config before the stored type is known,
// and the type is what decides whether the policy survives the API.
func rejectDeliveryPolicyInConfigForCLI(destType string, config map[string]interface{}, flagPrefix string) error {
	policy, ok := config["delivery_policy"].(map[string]interface{})
	if !ok {
		return nil
	}
	return rejectDeliveryPolicyForCLI(destType, policy, flagPrefix)
}

func mergeDeliveryPolicy(config map[string]interface{}, policy map[string]interface{}) {
	if len(policy) == 0 {
		return
	}
	merged := make(map[string]interface{})
	if existing, ok := config["delivery_policy"].(map[string]interface{}); ok {
		for key, value := range existing {
			merged[key] = value
		}
	}
	for key, value := range policy {
		merged[key] = value
	}
	config["delivery_policy"] = merged
}

// buildDestinationAuthConfig builds auth section for destination config from flags.
func buildDestinationAuthConfig(f *destinationConfigFlags) (map[string]interface{}, error) {
	if f == nil || f.AuthMethod == "" || f.AuthMethod == "hookdeck" {
		return nil, nil
	}
	auth := make(map[string]interface{})
	switch f.AuthMethod {
	case "bearer":
		if f.BearerToken == "" {
			return nil, fmt.Errorf("--bearer-token is required for bearer auth method")
		}
		auth["type"] = "BEARER_TOKEN"
		auth["token"] = f.BearerToken
	case "basic":
		if f.BasicAuthUser == "" || f.BasicAuthPass == "" {
			return nil, fmt.Errorf("--basic-auth-user and --basic-auth-pass are required for basic auth method")
		}
		auth["type"] = "BASIC_AUTH"
		auth["username"] = f.BasicAuthUser
		auth["password"] = f.BasicAuthPass
	case "api_key":
		if f.APIKey == "" {
			return nil, fmt.Errorf("--api-key is required for api_key auth method")
		}
		if f.APIKeyHeader == "" {
			return nil, fmt.Errorf("--api-key-header is required for api_key auth method")
		}
		auth["type"] = "API_KEY"
		auth["api_key"] = f.APIKey
		auth["key"] = f.APIKeyHeader
		to := f.APIKeyTo
		if to == "" {
			to = "header"
		}
		auth["to"] = to
	case "custom_signature":
		if f.CustomSignatureSecret == "" {
			return nil, fmt.Errorf("--custom-signature-secret is required for custom_signature auth method")
		}
		if f.CustomSignatureKey == "" {
			return nil, fmt.Errorf("--custom-signature-key is required for custom_signature auth method")
		}
		auth["type"] = "CUSTOM_SIGNATURE"
		auth["signing_secret"] = f.CustomSignatureSecret
		auth["key"] = f.CustomSignatureKey
	default:
		return nil, fmt.Errorf("unsupported destination auth method: %s (supported: hookdeck, bearer, basic, api_key, custom_signature)", f.AuthMethod)
	}
	return auth, nil
}

// buildDestinationConfigFromIndividualFlags builds destination config from flags for the given type.
func buildDestinationConfigFromIndividualFlags(destType string, f *destinationConfigFlags) (map[string]interface{}, error) {
	if f == nil {
		return make(map[string]interface{}), nil
	}
	config := make(map[string]interface{})

	authConfig, err := buildDestinationAuthConfig(f)
	if err != nil {
		return nil, err
	}
	if len(authConfig) > 0 {
		config["auth_type"] = authConfig["type"]
		auth := make(map[string]interface{})
		for k, v := range authConfig {
			if k != "type" {
				auth[k] = v
			}
		}
		config["auth"] = auth
	}

	policy, err := buildDeliveryPolicy(
		f.RateLimit,
		f.RateLimitPeriod,
		f.DeliveryGroupKey,
		f.DeliveryGroupRate,
		f.DeliveryGroupRatePeriod,
		f.DeliveryGroupOverrides,
		"",
	)
	if err != nil {
		return nil, err
	}
	if err := rejectDeliveryPolicyForCLI(destType, policy, ""); err != nil {
		return nil, err
	}
	mergeDeliveryPolicy(config, policy)

	// A flag belonging to another type would otherwise be dropped by the switch
	// below without a word.
	if err := rejectTypeSpecificFlagsForOtherTypes(destType, f); err != nil {
		return nil, err
	}

	switch strings.ToUpper(destType) {
	case "HTTP":
		if f.URL != "" {
			config["url"] = f.URL
		}
		if f.PathForwardingDisabled != nil {
			config["path_forwarding_disabled"] = *f.PathForwardingDisabled
		}
		if f.HTTPMethod != "" {
			valid := map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}
			method := strings.ToUpper(f.HTTPMethod)
			if !valid[method] {
				return nil, fmt.Errorf("--http-method must be one of: GET, POST, PUT, PATCH, DELETE")
			}
			config["http_method"] = method
		}
	case "CLI":
		if f.CliPath != "" {
			config["path"] = f.CliPath
		}
	case "MOCK_API":
		// no extra fields
	default:
		// An empty type stays tolerated here, because auth and delivery-policy
		// flags mean the same thing whatever the type and a typeless build is a
		// legitimate request for them. What cannot be tolerated is a type-
		// specific flag with no type to apply it to, and that is refused above.
		if destType != "" {
			return nil, fmt.Errorf("unsupported destination type: %s (supported: HTTP, CLI, MOCK_API)", destType)
		}
	}

	return config, nil
}

// buildDestinationConfigFromFlags parses destination config from --config/--config-file
// or from individual flags. When configStr or configFile is set, that takes precedence.
// destType is used when building from individual flags (HTTP requires url, etc.).
func buildDestinationConfigFromFlags(configStr, configFile, destType string, individual *destinationConfigFlags) (map[string]interface{}, error) {
	if configStr != "" {
		var out map[string]interface{}
		if err := json.Unmarshal([]byte(configStr), &out); err != nil {
			return nil, fmt.Errorf("invalid JSON in --config: %w", err)
		}
		return out, nil
	}
	if configFile != "" {
		data, err := os.ReadFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read --config-file: %w", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, fmt.Errorf("invalid JSON in config file: %w", err)
		}
		return out, nil
	}
	return buildDestinationConfigFromIndividualFlags(destType, individual)
}
