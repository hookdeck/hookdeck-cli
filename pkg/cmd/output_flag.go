package cmd

import (
	"fmt"

	"github.com/spf13/pflag"
)

// outputFormatValue is a pflag.Value for --output flags that only support
// "json". Unknown values are rejected at parse time instead of silently
// falling back to the human-readable output.
type outputFormatValue struct {
	target *string
}

func newOutputFormatValue(target *string) *outputFormatValue {
	return &outputFormatValue{target: target}
}

func (v *outputFormatValue) String() string {
	if v.target == nil {
		return ""
	}
	return *v.target
}

func (v *outputFormatValue) Set(s string) error {
	if s != "" && s != "json" {
		return fmt.Errorf("unsupported output format %q (supported: json)", s)
	}
	*v.target = s
	return nil
}

func (v *outputFormatValue) Type() string { return "string" }

var _ pflag.Value = (*outputFormatValue)(nil)
