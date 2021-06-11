package cmd

import (
	"github.com/spf13/cobra"

	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/logger"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// NewSecurityCmd : "cbadm securitygroup *" (for CB-Tumblebug)
func NewSecurityCmd() *cobra.Command {

	securityCmd := &cobra.Command{
		Use:     "securitygroup",
		Aliases: []string{"sg"},
		Short:   "This is a manageable command for securitygroup",
		Long:    "This is a manageable command for securitygroup",
	}

	//  Adds the commands for application.
	securityCmd.AddCommand(NewSecurityCreateCmd())
	securityCmd.AddCommand(NewSecurityListCmd())
	securityCmd.AddCommand(NewSecurityListIdCmd())
	securityCmd.AddCommand(NewSecurityGetCmd())
	securityCmd.AddCommand(NewSecurityDeleteCmd())
	securityCmd.AddCommand(NewSecurityDeleteAllCmd())

	return securityCmd
}

// NewSecurityCreateCmd : "cbadm securitygroup create"
func NewSecurityCreateCmd() *cobra.Command {

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "This is create command for securitygroup",
		Long:  "This is create command for securitygroup",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			readInDataFromFile()
			if inData == "" {
				logger.Error("failed to validate --indata parameter")
				return
			}
			logger.Debug("--indata parameter value : \n", inData)
			logger.Debug("--infile parameter value : ", inFile)

			SetupAndRun(cmd, args)
		},
	}

	createCmd.PersistentFlags().StringVarP(&inData, "indata", "d", "", "input string data")
	createCmd.PersistentFlags().StringVarP(&inFile, "infile", "f", "", "input file path")

	return createCmd
}

// NewSecurityListCmd : "cbadm securitygroup list"
func NewSecurityListCmd() *cobra.Command {

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "This is list command for securitygroup",
		Long:  "This is list command for securitygroup",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)

			SetupAndRun(cmd, args)
		},
	}

	listCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")

	return listCmd
}

// NewSecurityListIdCmd : "cbadm securitygroup list-id"
func NewSecurityListIdCmd() *cobra.Command {

	listIdCmd := &cobra.Command{
		Use:   "list-id",
		Short: "This is list-id command for securitygroup",
		Long:  "This is list-id command for securitygroup",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)

			SetupAndRun(cmd, args)
		},
	}

	listIdCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")

	return listIdCmd
}

// NewSecurityGetCmd : "cbadm securitygroup get"
func NewSecurityGetCmd() *cobra.Command {

	getCmd := &cobra.Command{
		Use:   "get",
		Short: "This is get command for securitygroup",
		Long:  "This is get command for securitygroup",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			if resourceID == "" {
				logger.Error("failed to validate --id parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--id parameter value : ", resourceID)

			SetupAndRun(cmd, args)
		},
	}

	getCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	getCmd.PersistentFlags().StringVarP(&resourceID, "id", "", "", "security id")

	return getCmd
}

// NewSecurityDeleteCmd : "cbadm securitygroup delete"
func NewSecurityDeleteCmd() *cobra.Command {

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "This is delete command for securitygroup",
		Long:  "This is delete command for securitygroup",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			if resourceID == "" {
				logger.Error("failed to validate --id parameter")
				return
			}
			if force == "" {
				logger.Error("failed to validate --force parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--id parameter value : ", resourceID)
			logger.Debug("--force parameter value : ", force)

			SetupAndRun(cmd, args)
		},
	}

	deleteCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	deleteCmd.PersistentFlags().StringVarP(&resourceID, "id", "", "", "security id")
	deleteCmd.PersistentFlags().StringVarP(&force, "force", "", "false", "force flag")

	return deleteCmd
}

// NewSecurityDeleteAllCmd : "cbadm securitygroup delete-all"
func NewSecurityDeleteAllCmd() *cobra.Command {

	deleteAllCmd := &cobra.Command{
		Use:   "delete-all",
		Short: "This is delete-all command for securitygroup",
		Long:  "This is delete-all command for securitygroup",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			if force == "" {
				logger.Error("failed to validate --force parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--force parameter value : ", force)

			SetupAndRun(cmd, args)
		},
	}

	deleteAllCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	deleteAllCmd.PersistentFlags().StringVarP(&force, "force", "", "false", "force flag")

	return deleteAllCmd
}
