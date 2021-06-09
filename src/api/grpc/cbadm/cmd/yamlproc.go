package cmd

import (
	"github.com/spf13/cobra"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// NewYamlApplyCmd : "cbadm apply" (create/update objects according to YAML description)
func NewYamlApplyCmd() *cobra.Command {

	yamlApplyCmd := &cobra.Command{
		Use:   "apply",
		Short: "This is a apply command for yaml",
		Long:  "This is a apply command for yaml",
		Run: func(cmd *cobra.Command, args []string) {
			SetupAndRun(cmd, args)
		},
	}

	return yamlApplyCmd
}

// NewYamlGetCmd : "cbadm get" (get objects according to YAML description)
func NewYamlGetCmd() *cobra.Command {

	yamlGetCmd := &cobra.Command{
		Use:   "get",
		Short: "This is a get command for yaml",
		Long:  "This is a get command for yaml",
		Run: func(cmd *cobra.Command, args []string) {
			SetupAndRun(cmd, args)
		},
	}

	return yamlGetCmd
}

// NewYamlListCmd : "cbadm list" (list objects according to YAML description)
func NewYamlListCmd() *cobra.Command {

	yamlListCmd := &cobra.Command{
		Use:   "list",
		Short: "This is a list command for yaml",
		Long:  "This is a list command for yaml",
		Run: func(cmd *cobra.Command, args []string) {
			SetupAndRun(cmd, args)
		},
	}

	return yamlListCmd
}

// NewYamlRemoveCmd : "cbadm remove" (remove objects according to YAML description)
func NewYamlRemoveCmd() *cobra.Command {

	yamlRemoveCmd := &cobra.Command{
		Use:     "remove",
		Aliases: []string{"rm", "delete"},
		Short:   "This is a remove command for yaml",
		Long:    "This is a remove command for yaml",
		Run: func(cmd *cobra.Command, args []string) {
			SetupAndRun(cmd, args)
		},
	}

	return yamlRemoveCmd
}
