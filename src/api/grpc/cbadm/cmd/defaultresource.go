package cmd

import (
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/logger"
	"github.com/spf13/cobra"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// NewDefaultResourceCmd : "cbadm defaultresource *" (for CB-Spider)
func NewDefaultResourceCmd() *cobra.Command {

	defaultResourceCmd := &cobra.Command{
		Use:   "defaultresource",
		Short: "This is a manageable command for default resources",
		Long:  "This is a manageable command for default resources",
	}

	//  Adds the commands for application.
	defaultResourceCmd.AddCommand(NewDefaultResourceLoadCmd())

	return defaultResourceCmd
}

// NewDefaultResourceLoadCmd : "cbadm defaultresource load"
func NewDefaultResourceLoadCmd() *cobra.Command {

	loadCmd := &cobra.Command{
		Use:   "load",
		Short: "Load default resources into the namespace 'default'.",
		Long:  "Load default resources into the namespace 'default'.",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			} else if resourceType == "" {
				logger.Error("failed to validate --resourceType parameter")
				return
			} //else if connConfigName == "" {
			// 	logger.Error("failed to validate --connConfigName parameter")
			// 	return
			// }
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--resourceType parameter value : ", resourceType)
			logger.Debug("--connConfigName parameter value : ", connConfigName)

			SetupAndRun(cmd, args)
		},
	}

	loadCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	loadCmd.PersistentFlags().StringVarP(&resourceType, "resourceType", "", "", "resource type [all, vnet, sg, sshkey]")
	loadCmd.PersistentFlags().StringVarP(&connConfigName, "connConfigName", "", "", "connection config name (optional)")

	return loadCmd
}
