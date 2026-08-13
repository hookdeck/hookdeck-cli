package cmd

import "fmt"

// confirmDestructiveAction asks the user to confirm a destructive operation.
//
// It returns (true, nil) to proceed, (false, nil) when the user declined — the
// caller should return nil, this is a successful no-op — and (false, err) when
// confirmation is impossible because no terminal is attached.
//
// That last case is the point. These commands previously called fmt.Scanln and
// discarded its error, so without a terminal the response stayed empty, the
// command printed "Deletion cancelled." and returned nil. A CI job that deleted
// nothing therefore exited 0 and looked like a success. Every one of these
// commands already has a --force flag, so the non-interactive path exists; it
// just was not required.
func confirmDestructiveAction(prompt, cancelledMessage, forceFlagName string) (bool, error) {
	if !stdinIsTerminal() {
		return false, fmt.Errorf(
			"cannot confirm this action: no terminal is attached.\n"+
				"Re-run with --%s to confirm, or run it in an interactive terminal.",
			forceFlagName)
	}

	fmt.Printf("%s [y/N]: ", prompt)

	var response string
	// Error intentionally ignored: a bare Enter yields "unexpected newline" and
	// an empty response, which is a decline — the correct reading of [y/N].
	fmt.Scanln(&response)

	if response != "y" && response != "Y" {
		fmt.Println(cancelledMessage)
		return false, nil
	}

	return true, nil
}
