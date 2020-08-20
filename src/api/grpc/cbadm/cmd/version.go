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

// NewVersionCmd - 버전 표시 기능을 수행하는 Cobra Command 생성
func NewVersionCmd() *cobra.Command {

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "This is a version command for cbadm",
		Long:  "This is a version command for cbadm",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("CBADM CLI VERSION %s\n", CLIVersion)
		},
	}

	return versionCmd
}
