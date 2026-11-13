package cmd

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// storedWithSides is the connection every guard test holds as the stored one.
func storedWithSides() []hookdeck.Connection {
	return []hookdeck.Connection{*storedUpsertConnection()}
}

// TestConnectionUpsertRejectsChangedSide covers each way of naming a different
// source or destination. Every case has to fail before any write: a PUT of the
// connection, or of an inline destination, is what made the change look
// applied while the API ignored it (#466).
func TestConnectionUpsertRejectsChangedSide(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		message string
	}{
		{
			name:    "inline destination with a different name",
			args:    []string{"--source-name", "my-src", "--source-type", "WEBHOOK", "--destination-name", "other-api", "--destination-type", "HTTP", "--destination-url", "https://other.example.com"},
			message: "destination",
		},
		{
			name:    "different destination ID with the stored source ID",
			args:    []string{"--source-id", "src_1", "--destination-id", "des_other"},
			message: "destination",
		},
		{
			name:    "different destination ID alone",
			args:    []string{"--destination-id", "des_other"},
			message: "destination",
		},
	}

	for _, tc := range cases {
		for _, dryRun := range []bool{false, true} {
			tc, dryRun := tc, dryRun
			name := tc.name
			args := append([]string{"gateway", "connection", "upsert", "my-conn"}, tc.args...)
			if dryRun {
				name += " (dry-run)"
				args = append(args, "--dry-run")
			}
			t.Run(name, func(t *testing.T) {
				stub, err := runConnectionCommand(t, storedWithSides(), args...)

				require.Error(t, err)
				require.Contains(t, err.Error(), "cannot be changed")
				require.Contains(t, err.Error(), tc.message)
				require.Empty(t, stub.writes(), "no write may be made once the change is rejected")
			})
		}
	}
}

// TestConnectionUpsertWithADifferentSourceIsADifferentConnection pins what the
// live API does: a connection name is only unique per source, so naming another
// source creates a second connection with the same name instead of re-pointing
// the stored one. That connection may have any destination, so the destination
// guard must not apply to it - the live API accepts this, and refusing it would
// block a working feature.
func TestConnectionUpsertWithADifferentSourceIsADifferentConnection(t *testing.T) {
	t.Run("by ID, with a different destination", func(t *testing.T) {
		stub, err := runConnectionCommand(t, storedWithSides(),
			"gateway", "connection", "upsert", "my-conn", "--source-id", "src_other", "--destination-id", "des_other")

		require.NoError(t, err)
		require.Len(t, stub.writes(), 1)
		require.Equal(t, "src_other", stub.writes()[0].Body["source_id"])
		require.Equal(t, "des_other", stub.writes()[0].Body["destination_id"])
	})

	t.Run("by inline name", func(t *testing.T) {
		stub, err := runConnectionCommand(t, storedWithSides(),
			"gateway", "connection", "upsert", "my-conn", "--source-name", "other-src")

		require.NoError(t, err)
		require.Len(t, stub.writes(), 1)
		source, ok := stub.writes()[0].Body["source"].(map[string]interface{})
		require.True(t, ok, "the new source must be sent inline")
		require.Equal(t, "other-src", source["name"])
	})
}

// twoConnectionsNamedMyConn returns my-conn on two sources, each with its own
// destination.
func twoConnectionsNamedMyConn() []hookdeck.Connection {
	first := *storedUpsertConnection()
	second := *storedUpsertConnection()
	second.ID = "web_2"
	second.Source = &hookdeck.Source{ID: "src_2", Name: "second-src", Type: "WEBHOOK"}
	second.Destination = &hookdeck.Destination{ID: "des_2", Name: "second-api", Type: "HTTP"}
	return []hookdeck.Connection{first, second}
}

// TestConnectionUpsertAmbiguousName: the same name on two sources is two
// connections, and picking the first would update the wrong one.
func TestConnectionUpsertAmbiguousName(t *testing.T) {
	t.Run("no source named is refused before any write", func(t *testing.T) {
		stub, err := runConnectionCommand(t, twoConnectionsNamedMyConn(),
			"gateway", "connection", "upsert", "my-conn", "--description", "d")

		require.Error(t, err)
		require.Contains(t, err.Error(), "--source-id")
		require.Empty(t, stub.writes())
	})

	t.Run("the source picks the connection and its destination is guarded", func(t *testing.T) {
		stub, err := runConnectionCommand(t, twoConnectionsNamedMyConn(),
			"gateway", "connection", "upsert", "my-conn", "--source-id", "src_2", "--destination-id", "des_1")

		require.Error(t, err)
		require.Contains(t, err.Error(), "second-api", "the second connection's destination is the one being compared")
		require.Empty(t, stub.writes())
	})

	t.Run("negative control: the second connection's own destination is accepted", func(t *testing.T) {
		stub, err := runConnectionCommand(t, twoConnectionsNamedMyConn(),
			"gateway", "connection", "upsert", "my-conn", "--source-id", "src_2", "--destination-id", "des_2")

		require.NoError(t, err)
		require.Len(t, stub.writes(), 1)
	})
}

// TestConnectionUpsertAllowsUnchangedSides is the negative control: the same
// flags with the stored names and IDs must still reach the API, so the guard is
// rejecting the difference and not every upsert that names a destination.
func TestConnectionUpsertAllowsUnchangedSides(t *testing.T) {
	cases := map[string][]string{
		"inline sides with the stored names": {"--source-name", "my-src", "--source-type", "WEBHOOK", "--destination-name", "my-api", "--destination-type", "HTTP", "--destination-url", "https://api.example.com/v2"},
		"the stored IDs":                     {"--source-id", "src_1", "--destination-id", "des_1"},
	}
	for name, flags := range cases {
		flags := flags
		t.Run(name, func(t *testing.T) {
			args := append([]string{"gateway", "connection", "upsert", "my-conn"}, flags...)
			stub, err := runConnectionCommand(t, storedWithSides(), args...)

			require.NoError(t, err)
			require.Len(t, stub.writes(), 1)
			require.Equal(t, http.MethodPut, stub.writes()[0].Method)
		})
	}
}

// TestConnectionUpsertChangedSideOnNewConnectionIsACreate: when no connection
// has the name there is nothing to compare with, and the flags create one.
func TestConnectionUpsertChangedSideOnNewConnectionIsACreate(t *testing.T) {
	stub, err := runConnectionCommand(t, nil,
		"gateway", "connection", "upsert", "brand-new",
		"--source-id", "src_x", "--destination-id", "des_x")

	require.NoError(t, err)
	require.Len(t, stub.writes(), 1)
	require.Equal(t, "des_x", stub.writes()[0].Body["destination_id"])
}

// TestConnectionUpdateRejectsChangedSide covers connection update, which sent
// the new ID to an endpoint that discards it. Unlike upsert, update ignores a
// new source as well as a new destination (verified against the live API).
func TestConnectionUpdateRejectsChangedSide(t *testing.T) {
	for flag, message := range map[string]string{
		"--destination-id": "destination",
		"--source-id":      "source",
	} {
		flag, message := flag, message
		t.Run(flag, func(t *testing.T) {
			stub, err := runConnectionCommand(t, storedWithSides(),
				"gateway", "connection", "update", "web_1", flag, "other_id")

			require.Error(t, err)
			require.Contains(t, err.Error(), "cannot be changed")
			require.Contains(t, err.Error(), message)
			require.Empty(t, stub.writes())
		})
	}
}

// TestConnectionUpdateAcceptsStoredSideIDs is the negative control for update:
// the stored IDs are accepted, are not sent, and other changes still go out.
func TestConnectionUpdateAcceptsStoredSideIDs(t *testing.T) {
	stub, err := runConnectionCommand(t, storedWithSides(),
		"gateway", "connection", "update", "web_1",
		"--destination-id", "des_1", "--source-id", "src_1", "--description", "new")

	require.NoError(t, err)
	require.Len(t, stub.writes(), 1)
	body := stub.writes()[0].Body
	require.Equal(t, "new", body["description"])
	require.NotContains(t, body, "destination_id")
	require.NotContains(t, body, "source_id")
}
