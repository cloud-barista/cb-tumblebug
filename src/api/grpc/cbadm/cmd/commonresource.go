package cmd

import (
	"github.com/spf13/cobra"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// NewCommonResourceCmd : "cbadm commonresource *" (for CB-Spider)
func NewCommonResourceCmd() *cobra.Command {

	commonResourceCmd := &cobra.Command{
		Use:   "commonresource",
		Short: "This is a manageable command for common resources",
		Long:  "This is a manageable command for common resources",
	}

	//  Adds the commands for application.
	commonResourceCmd.AddCommand(NewCommonResourceLoadCmd())

	return commonResourceCmd
}

// NewCommonResourceLoadCmd : "cbadm commonresource load"
func NewCommonResourceLoadCmd() *cobra.Command {

	createCmd := &cobra.Command{
		Use:   "load",
		Short: "Load common resources into the namespace 'common'.",
		Long:  "Load common resources into the namespace 'common'.",
		Run: func(cmd *cobra.Command, args []string) {
			SetupAndRun(cmd, args)
		},
	}

	return createCmd
}
