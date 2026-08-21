//go:build basic

package acceptance

import "testing"

// The cleanup bookkeeping reads command arguments and command output, so it can
// be wrong in ways no acceptance run would report: a missed create leaks quietly
// and a mis-parsed noun deletes the wrong kind of resource. These are unit tests
// over the parsing alone — no CLI, no API.

func TestGatewayCommandForTracking(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantOK   bool
		wantNoun trackedKind
		wantVerb string
		wantOps  []string
	}{
		{
			name:     "gateway connection create",
			args:     []string{"gateway", "connection", "create", "--name", "x", "--output", "json"},
			wantOK:   true,
			wantNoun: kindConnection,
			wantVerb: "create",
			wantOps:  []string{"--name", "x", "--output", "json"},
		},
		{
			name:     "root alias without gateway",
			args:     []string{"connection", "delete", "web_abc", "--force"},
			wantOK:   true,
			wantNoun: kindConnection,
			wantVerb: "delete",
			wantOps:  []string{"web_abc", "--force"},
		},
		{
			name:     "global flags before the noun",
			args:     []string{"--api-base", "http://127.0.0.1:1", "gateway", "source", "create", "--name", "s"},
			wantOK:   true,
			wantNoun: kindSource,
			wantVerb: "create",
			wantOps:  []string{"--name", "s"},
		},
		{
			// An outpost destination is not a gateway destination, and deleting
			// it with `gateway destination delete` would be the wrong call.
			name:   "outpost commands are not gateway commands",
			args:   []string{"outpost", "destination", "create", "--tenant-id", "t"},
			wantOK: false,
		},
		{
			// A flag value can be any word; only position tells them apart.
			name:   "flag value that looks like a noun",
			args:   []string{"gateway", "event", "list", "--filter", "source", "create"},
			wantOK: false,
		},
		{
			name:   "no verb after the noun",
			args:   []string{"gateway", "source"},
			wantOK: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			noun, verb, operands, ok := gatewayCommandForTracking(tc.args)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if noun != tc.wantNoun || verb != tc.wantVerb {
				t.Fatalf("got %s %s, want %s %s", noun, verb, tc.wantNoun, tc.wantVerb)
			}
			if len(operands) != len(tc.wantOps) {
				t.Fatalf("operands = %v, want %v", operands, tc.wantOps)
			}
			for i := range operands {
				if operands[i] != tc.wantOps[i] {
					t.Fatalf("operands = %v, want %v", operands, tc.wantOps)
				}
			}
		})
	}
}

func TestCreatedResourcesFromOutput(t *testing.T) {
	t.Run("json create records all three resources", func(t *testing.T) {
		stdout := `{"id":"web_1","name":"c","source":{"id":"src_1"},"destination":{"id":"des_1"}}`
		got := createdResourcesFromOutput(kindConnection, stdout)
		want := []trackedResource{
			{kind: kindConnection, id: "web_1"},
			{kind: kindSource, id: "src_1"},
			{kind: kindDestination, id: "des_1"},
		}
		assertResources(t, got, want)
	})

	t.Run("warning printed before the json body", func(t *testing.T) {
		// Seen in a real run: a rate-limited spec download prints a warning line,
		// the test's own parse fails, and the resources it created must still be
		// deleted.
		stdout := "Warning: could not fetch source types for validation\n" +
			`{"id":"web_2","source":{"id":"src_2"},"destination":{"id":"des_2"}}`
		got := createdResourcesFromOutput(kindConnection, stdout)
		want := []trackedResource{
			{kind: kindConnection, id: "web_2"},
			{kind: kindSource, id: "src_2"},
			{kind: kindDestination, id: "des_2"},
		}
		assertResources(t, got, want)
	})

	t.Run("human readable create output", func(t *testing.T) {
		stdout := "✓ Connection created successfully\n\n" +
			"Connection:  my-conn (web_3)\n" +
			"Source:      my-src (src_3)\n" +
			"Destination: my-dst (des_3)\n"
		got := createdResourcesFromOutput(kindConnection, stdout)
		want := []trackedResource{
			{kind: kindConnection, id: "web_3"},
			{kind: kindSource, id: "src_3"},
			{kind: kindDestination, id: "des_3"},
		}
		assertResources(t, got, want)
	})

	t.Run("ids quoted in prose are not resources", func(t *testing.T) {
		// Error hints quote the id the user passed. Nothing was created, and
		// deleting a bystander's id would be worse than leaking one.
		stdout := "failed to create connection\nHints:\n  --source-id src_nonexistent123 was not found\n"
		if got := createdResourcesFromOutput(kindConnection, stdout); len(got) != 0 {
			t.Fatalf("got %v, want none", got)
		}
	})
}

func TestListenSourceNames(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{name: "port and source", args: []string{"listen", "8080", "test-src"}, want: []string{"test-src"}},
		{name: "with flags after", args: []string{"listen", "8080", "test-src", "--output", "compact"}, want: []string{"test-src"}},
		{name: "with a path", args: []string{"listen", "8080", "test-src", "/webhooks"}, want: []string{"test-src"}},
		{name: "comma separated", args: []string{"listen", "8080", "a,b"}, want: []string{"a", "b"}},
		{name: "global flags first", args: []string{"--api-base", "http://x", "listen", "9999", "test-src"}, want: []string{"test-src"}},
		// A wildcard listens to sources that already exist and creates none.
		{name: "wildcard creates nothing", args: []string{"listen", "8080", "*"}, want: nil},
		{name: "port only creates nothing", args: []string{"listen", "8080"}, want: nil},
		{name: "not a listen command", args: []string{"gateway", "source", "list"}, want: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := listenSourceNames(tc.args)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func assertResources(t *testing.T, got, want []trackedResource) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
