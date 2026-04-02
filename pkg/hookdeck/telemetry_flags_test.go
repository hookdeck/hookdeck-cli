package hookdeck

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestCollectChangedFlagNames(t *testing.T) {
	root := &cobra.Command{Use: "hookdeck"}
	root.PersistentFlags().String("cli-key", "", "")
	root.PersistentFlags().String("api-key", "", "")
	login := &cobra.Command{
		Use: "login",
		Run: func(cmd *cobra.Command, args []string) {},
	}
	login.Flags().Bool("local", false, "")
	login.Flags().BoolP("interactive", "i", false, "")
	root.AddCommand(login)

	root.SetArgs([]string{"login", "--cli-key", "secret", "--local"})
	cmd, err := root.ExecuteC()
	require.NoError(t, err)
	names := CollectChangedFlagNames(cmd)
	require.ElementsMatch(t, []string{"cli-key", "local"}, names)
}

func TestCollectChangedFlagNames_apiKeyAlias(t *testing.T) {
	root := &cobra.Command{Use: "hookdeck"}
	root.PersistentFlags().String("api-key", "", "")
	login := &cobra.Command{Use: "login", Run: func(*cobra.Command, []string) {}}
	root.AddCommand(login)
	root.SetArgs([]string{"login", "--api-key", "k"})
	cmd, err := root.ExecuteC()
	require.NoError(t, err)
	require.Equal(t, []string{"api-key"}, CollectChangedFlagNames(cmd))
}

func TestCollectChangedFlagNames_nilCommand(t *testing.T) {
	require.Nil(t, CollectChangedFlagNames(nil))
}
