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

// NewImageCmd - Image 관리 기능을 수행하는 Cobra Command 생성
func NewImageCmd() *cobra.Command {

	imageCmd := &cobra.Command{
		Use:   "images",
		Short: "This is a manageable command for image",
		Long:  "This is a manageable command for image",
	}

	//  Adds the commands for application.
	imageCmd.AddCommand(NewImageCreateWithInfoCmd())
	imageCmd.AddCommand(NewImageListCmd())
	imageCmd.AddCommand(NewImageGetCmd())
	imageCmd.AddCommand(NewImageDeleteCmd())

	return imageCmd
}

// NewImageCreateWithInfoCmd - Image 생성 기능을 수행하는 Cobra Command 생성
func NewImageCreateWithInfoCmd() *cobra.Command {

	createWithInfoCmd := &cobra.Command{
		Use:   "create",
		Short: "This is create command for image",
		Long:  "This is create command for image",
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

	createWithInfoCmd.PersistentFlags().StringVarP(&inData, "indata", "d", "", "input string data")
	createWithInfoCmd.PersistentFlags().StringVarP(&inFile, "infile", "f", "", "input file path")

	return createWithInfoCmd
}

// NewImageListCmd - Image 목록 기능을 수행하는 Cobra Command 생성
func NewImageListCmd() *cobra.Command {

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "This is list command for image",
		Long:  "This is list command for image",
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

// NewImageGetCmd - Image 조회 기능을 수행하는 Cobra Command 생성
func NewImageGetCmd() *cobra.Command {

	getCmd := &cobra.Command{
		Use:   "get",
		Short: "This is get command for image",
		Long:  "This is get command for image",
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
	getCmd.PersistentFlags().StringVarP(&resourceID, "id", "", "", "image id")

	return getCmd
}

// NewImageDeleteCmd - Image 삭제 기능을 수행하는 Cobra Command 생성
func NewImageDeleteCmd() *cobra.Command {

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "This is delete command for image",
		Long:  "This is delete command for image",
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
	deleteCmd.PersistentFlags().StringVarP(&resourceID, "id", "", "", "image id")
	deleteCmd.PersistentFlags().StringVarP(&force, "force", "", "false", "force flag")

	return deleteCmd
}
