package hookdeck

import "fmt"

// CheckConnectionSidesUnchanged returns an error when a requested source or
// destination differs from the one the connection already has.
//
// The API never re-points an existing connection: a PUT with a different
// source_id or destination_id, or an inline destination with a different name,
// is accepted and the stored side is left as it was. The caller would then see
// success for a change that did not happen, and an inline destination would be
// upserted and left attached to nothing. Run this before any write.
//
// Each side is identified by ID or by name; an empty value means that side was
// not mentioned and is not checked. Names are compared exactly, as they are
// when the API resolves them.
func CheckConnectionSidesUnchanged(existing *Connection, sourceID, sourceName, destinationID, destinationName string) error {
	if existing == nil {
		return nil
	}
	label := existing.ID
	if existing.Name != nil {
		label = fmt.Sprintf("%s (%s)", *existing.Name, existing.ID)
	}

	if existing.Source != nil {
		if sourceID != "" && sourceID != existing.Source.ID {
			return sideChangeError("source", label, existing.Source.Name, existing.Source.ID, sourceID)
		}
		if sourceName != "" && sourceName != existing.Source.Name {
			return sideChangeError("source", label, existing.Source.Name, existing.Source.ID, sourceName)
		}
	}
	if existing.Destination != nil {
		if destinationID != "" && destinationID != existing.Destination.ID {
			return sideChangeError("destination", label, existing.Destination.Name, existing.Destination.ID, destinationID)
		}
		if destinationName != "" && destinationName != existing.Destination.Name {
			return sideChangeError("destination", label, existing.Destination.Name, existing.Destination.ID, destinationName)
		}
	}
	return nil
}

func sideChangeError(side, connection, currentName, currentID, requested string) error {
	return fmt.Errorf(
		"connection %s already uses %s %s (%s), and a connection's %s cannot be changed; %q was not applied and nothing was created or modified. "+
			"To use a different %s, create a new connection, or delete this one and recreate it",
		connection, side, currentName, currentID, side, requested, side)
}
