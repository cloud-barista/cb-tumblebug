package cmd

import (
	"github.com/spf13/cobra"

	"github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/logger"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// NewCredentialCmd : "cbadm credential *" (for CB-Spider)
func NewCredentialCmd() *cobra.Command {

	credentialCmd := &cobra.Command{
		Use:   "credential",
		Short: "This is a manageable command for credential",
		Long:  "This is a manageable command for credential",
	}

	//  Adds the commands for application.
	credentialCmd.AddCommand(NewCredentialCreateCmd())
	credentialCmd.AddCommand(NewCredentialListCmd())
	credentialCmd.AddCommand(NewCredentialGetCmd())
	credentialCmd.AddCommand(NewCredentialDeleteCmd())

	return credentialCmd
}

// NewCredentialCreateCmd : "cbadm credential create"
func NewCredentialCreateCmd() *cobra.Command {

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "This is create command for credential",
		Long:  "This is create command for credential",
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

// NewCredentialListCmd : "cbadm credential list"
func NewCredentialListCmd() *cobra.Command {

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "This is list command for credential",
		Long:  "This is list command for credential",
		Run: func(cmd *cobra.Command, args []string) {
			SetupAndRun(cmd, args)
		},
	}

	return listCmd
}

// NewCredentialGetCmd : "cbadm credential get"
func NewCredentialGetCmd() *cobra.Command {

	getCmd := &cobra.Command{
		Use:   "get",
		Short: "This is get command for credential",
		Long:  "This is get command for credential",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if credentialName == "" {
				logger.Error("failed to validate --name parameter")
				return
			}
			logger.Debug("--name parameter value : ", credentialName)

			SetupAndRun(cmd, args)
		},
	}

	getCmd.PersistentFlags().StringVarP(&credentialName, "name", "n", "", "crendential name")

	return getCmd
}

// NewCredentialDeleteCmd : "cbadm credential delete"
func NewCredentialDeleteCmd() *cobra.Command {

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "This is delete command for credential",
		Long:  "This is delete command for credential",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if credentialName == "" {
				logger.Error("failed to validate --name parameter")
				return
			}
			logger.Debug("--name parameter value : ", credentialName)

			SetupAndRun(cmd, args)
		},
	}

	deleteCmd.PersistentFlags().StringVarP(&credentialName, "name", "n", "", "crendential name")

	return deleteCmd
}
