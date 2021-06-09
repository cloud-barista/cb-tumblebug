package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// NewVersionCmd : "cbadm version"
func NewVersionCmd() *cobra.Command {

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "This is a version command for cbadm",
		Long:  "This is a version command for cbadm",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("cbadm cli version %s\n", CLIVersion)
		},
	}

	return versionCmd
}
