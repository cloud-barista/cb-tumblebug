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

// NewKeypairCmd - Keypair 관리 기능을 수행하는 Cobra Command 생성
func NewKeypairCmd() *cobra.Command {

	keypairCmd := &cobra.Command{
		Use:   "keypair",
		Short: "This is a manageable command for keypair",
		Long:  "This is a manageable command for keypair",
	}

	//  Adds the commands for application.
	keypairCmd.AddCommand(NewKeypairCreateCmd())
	keypairCmd.AddCommand(NewKeypairListCmd())
	keypairCmd.AddCommand(NewKeypairGetCmd())
	keypairCmd.AddCommand(NewKeypairSaveCmd())
	keypairCmd.AddCommand(NewKeypairDeleteCmd())

	return keypairCmd
}

// NewKeypairCreateCmd - Keypair 생성 기능을 수행하는 Cobra Command 생성
func NewKeypairCreateCmd() *cobra.Command {

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "This is create command for keypair",
		Long:  "This is create command for keypair",
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

// NewKeypairListCmd - Keypair 목록 기능을 수행하는 Cobra Command 생성
func NewKeypairListCmd() *cobra.Command {

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "This is list command for keypair",
		Long:  "This is list command for keypair",
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

// NewKeypairGetCmd - Keypair 조회 기능을 수행하는 Cobra Command 생성
func NewKeypairGetCmd() *cobra.Command {

	getCmd := &cobra.Command{
		Use:   "get",
		Short: "This is get command for keypair",
		Long:  "This is get command for keypair",
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
	getCmd.PersistentFlags().StringVarP(&resourceID, "id", "", "", "keypair id")

	return getCmd
}

// NewKeypairSaveCmd - Keypair 저장 기능을 수행하는 Cobra Command 생성
func NewKeypairSaveCmd() *cobra.Command {

	saveCmd := &cobra.Command{
		Use:   "save",
		Short: "This is save command for keypair",
		Long:  "This is save command for keypair",
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
			if sshSaveFileName == "" {
				logger.Error("failed to validate --fn parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--id parameter value : ", resourceID)
			logger.Debug("--fn parameter value : ", sshSaveFileName)

			SetupAndRun(cmd, args)
		},
	}

	saveCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	saveCmd.PersistentFlags().StringVarP(&resourceID, "id", "", "", "keypair id")
	saveCmd.PersistentFlags().StringVarP(&sshSaveFileName, "fn", "", "", "ssh key save file name")

	return saveCmd
}

// NewKeypairDeleteCmd - Keypair 삭제 기능을 수행하는 Cobra Command 생성
func NewKeypairDeleteCmd() *cobra.Command {

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "This is delete command for keypair",
		Long:  "This is delete command for keypair",
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
	deleteCmd.PersistentFlags().StringVarP(&resourceID, "id", "", "", "keypair id")
	deleteCmd.PersistentFlags().StringVarP(&force, "force", "", "false", "force flag")

	return deleteCmd
}
