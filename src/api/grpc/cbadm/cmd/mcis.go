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

// NewMcisCmd - Mcis 관리 기능을 수행하는 Cobra Command 생성
func NewMcisCmd() *cobra.Command {

	mcisCmd := &cobra.Command{
		Use:   "mcis",
		Short: "This is a manageable command for mcis",
		Long:  "This is a manageable command for mcis",
	}

	//  Adds the commands for application.
	mcisCmd.AddCommand(NewMcisCreateCmd())
	mcisCmd.AddCommand(NewMcisListCmd())
	mcisCmd.AddCommand(NewMcisGetCmd())
	mcisCmd.AddCommand(NewMcisDeleteCmd())
	mcisCmd.AddCommand(NewMcisStatusCmd())
	mcisCmd.AddCommand(NewMcisSuspendCmd())
	mcisCmd.AddCommand(NewMcisResumeCmd())
	mcisCmd.AddCommand(NewMcisRebootCmd())
	mcisCmd.AddCommand(NewMcisTerminateCmd())

	mcisCmd.AddCommand(NewMcisVmAddCmd())
	mcisCmd.AddCommand(NewMcisVmListCmd())
	mcisCmd.AddCommand(NewMcisVmGetCmd())
	mcisCmd.AddCommand(NewMcisVmDeleteCmd())
	mcisCmd.AddCommand(NewMcisVmStatusCmd())
	mcisCmd.AddCommand(NewMcisVmSuspendCmd())
	mcisCmd.AddCommand(NewMcisVmResumeCmd())
	mcisCmd.AddCommand(NewMcisVmRebootCmd())
	mcisCmd.AddCommand(NewMcisVmTerminateCmd())

	mcisCmd.AddCommand(NewCmdMcisCmd())
	mcisCmd.AddCommand(NewCmdMcisVmCmd())

	mcisCmd.AddCommand(NewDeployMilkywayCmd())

	mcisCmd.AddCommand(NewAccessVmCmd())
	mcisCmd.AddCommand(NewBenchmarkCmd())

	return mcisCmd
}

// NewMcisCreateCmd - Mcis 생성 기능을 수행하는 Cobra Command 생성
func NewMcisCreateCmd() *cobra.Command {

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "This is create command for mcis",
		Long:  "This is create command for mcis",
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

// NewMcisListCmd - Mcis 목록 기능을 수행하는 Cobra Command 생성
func NewMcisListCmd() *cobra.Command {

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "This is list command for mcis",
		Long:  "This is list command for mcis",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			if option == "" {
				logger.Error("failed to validate --option parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--option parameter value : ", option)

			SetupAndRun(cmd, args)
		},
	}

	listCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	listCmd.PersistentFlags().StringVarP(&option, "option", "", "", "status option")

	return listCmd
}

// NewMcisGetCmd - Mcis 조회 기능을 수행하는 Cobra Command 생성
func NewMcisGetCmd() *cobra.Command {

	getCmd := &cobra.Command{
		Use:   "get",
		Short: "This is get command for mcis",
		Long:  "This is get command for mcis",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			if mcisID == "" {
				logger.Error("failed to validate --mcis parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--mcis parameter value : ", mcisID)

			SetupAndRun(cmd, args)
		},
	}

	getCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	getCmd.PersistentFlags().StringVarP(&mcisID, "mcis", "", "", "mcis id")

	return getCmd
}

// NewMcisDeleteCmd - Mcis 삭제 기능을 수행하는 Cobra Command 생성
func NewMcisDeleteCmd() *cobra.Command {

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "This is delete command for mcis",
		Long:  "This is delete command for mcis",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			if mcisID == "" {
				logger.Error("failed to validate --mcis parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--mcis parameter value : ", mcisID)

			SetupAndRun(cmd, args)
		},
	}

	deleteCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	deleteCmd.PersistentFlags().StringVarP(&mcisID, "mcis", "", "", "mcis id")

	return deleteCmd
}

// NewMcisStatusCmd - Mcis 상태 조회 기능을 수행하는 Cobra Command 생성
func NewMcisStatusCmd() *cobra.Command {

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "This is status command for mcis",
		Long:  "This is status command for mcis",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			if mcisID == "" {
				logger.Error("failed to validate --mcis parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--mcis parameter value : ", mcisID)

			SetupAndRun(cmd, args)
		},
	}

	statusCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	statusCmd.PersistentFlags().StringVarP(&mcisID, "mcis", "", "", "mcis id")

	return statusCmd
}

// NewMcisSuspendCmd - Mcis Suspend 기능을 수행하는 Cobra Command 생성
func NewMcisSuspendCmd() *cobra.Command {

	suspendCmd := &cobra.Command{
		Use:   "suspend",
		Short: "This is suspend command for mcis",
		Long:  "This is suspend command for mcis",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			if mcisID == "" {
				logger.Error("failed to validate --mcis parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--mcis parameter value : ", mcisID)

			SetupAndRun(cmd, args)
		},
	}

	suspendCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	suspendCmd.PersistentFlags().StringVarP(&mcisID, "mcis", "", "", "mcis id")

	return suspendCmd
}

// NewMcisResumeCmd - Mcis Resume 기능을 수행하는 Cobra Command 생성
func NewMcisResumeCmd() *cobra.Command {

	resumeCmd := &cobra.Command{
		Use:   "resume",
		Short: "This is resume command for mcis",
		Long:  "This is resume command for mcis",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			if mcisID == "" {
				logger.Error("failed to validate --mcis parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--mcis parameter value : ", mcisID)

			SetupAndRun(cmd, args)
		},
	}

	resumeCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	resumeCmd.PersistentFlags().StringVarP(&mcisID, "mcis", "", "", "mcis id")

	return resumeCmd
}

// NewMcisRebootCmd - Mcis Reboot 기능을 수행하는 Cobra Command 생성
func NewMcisRebootCmd() *cobra.Command {

	rebootCmd := &cobra.Command{
		Use:   "reboot",
		Short: "This is reboot command for mcis",
		Long:  "This is reboot command for mcis",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			if mcisID == "" {
				logger.Error("failed to validate --mcis parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--mcis parameter value : ", mcisID)

			SetupAndRun(cmd, args)
		},
	}

	rebootCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	rebootCmd.PersistentFlags().StringVarP(&mcisID, "mcis", "", "", "mcis id")

	return rebootCmd
}

// NewMcisTerminateCmd - Mcis Terminate 기능을 수행하는 Cobra Command 생성
func NewMcisTerminateCmd() *cobra.Command {

	terminateCmd := &cobra.Command{
		Use:   "terminate",
		Short: "This is terminate command for mcis",
		Long:  "This is terminate command for mcis",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			if mcisID == "" {
				logger.Error("failed to validate --mcis parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--mcis parameter value : ", mcisID)

			SetupAndRun(cmd, args)
		},
	}

	terminateCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	terminateCmd.PersistentFlags().StringVarP(&mcisID, "mcis", "", "", "mcis id")

	return terminateCmd
}

// NewMcisVmAddCmd - Mcis VM 생성 기능을 수행하는 Cobra Command 생성
func NewMcisVmAddCmd() *cobra.Command {

	vmAddCmd := &cobra.Command{
		Use:   "add-vm",
		Short: "This is add-vm command for mcis",
		Long:  "This is add-vm command for mcis",
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

	vmAddCmd.PersistentFlags().StringVarP(&inData, "indata", "d", "", "input string data")
	vmAddCmd.PersistentFlags().StringVarP(&inFile, "infile", "f", "", "input file path")

	return vmAddCmd
}

// NewMcisVmListCmd - Mcis VM 목록 기능을 수행하는 Cobra Command 생성
func NewMcisVmListCmd() *cobra.Command {

	vmListCmd := &cobra.Command{
		Use:   "list-vm",
		Short: "This is list-vm command for mcis",
		Long:  "This is list-vm command for mcis",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			if mcisID == "" {
				logger.Error("failed to validate --mcis parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--mcis parameter value : ", mcisID)

			SetupAndRun(cmd, args)
		},
	}

	vmListCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	vmListCmd.PersistentFlags().StringVarP(&mcisID, "mcis", "", "", "mcis id")

	return vmListCmd
}

// NewMcisVmGetCmd - Mcis VM 조회 기능을 수행하는 Cobra Command 생성
func NewMcisVmGetCmd() *cobra.Command {

	vmGetCmd := &cobra.Command{
		Use:   "get-vm",
		Short: "This is get-vm command for mcis",
		Long:  "This is get-vm command for mcis",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			if mcisID == "" {
				logger.Error("failed to validate --mcis parameter")
				return
			}
			if vmID == "" {
				logger.Error("failed to validate --vm parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--mcis parameter value : ", mcisID)
			logger.Debug("--vm parameter value : ", vmID)

			SetupAndRun(cmd, args)
		},
	}

	vmGetCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	vmGetCmd.PersistentFlags().StringVarP(&mcisID, "mcis", "", "", "mcis id")
	vmGetCmd.PersistentFlags().StringVarP(&vmID, "vm", "", "", "mcis vm id")

	return vmGetCmd
}

// NewMcisVmDeleteCmd - Mcis VM 삭제 기능을 수행하는 Cobra Command 생성
func NewMcisVmDeleteCmd() *cobra.Command {

	vmDeleteCmd := &cobra.Command{
		Use:   "del-vm",
		Short: "This is del-vm command for mcis",
		Long:  "This is del-vm command for mcis",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			if mcisID == "" {
				logger.Error("failed to validate --mcis parameter")
				return
			}
			if vmID == "" {
				logger.Error("failed to validate --vm parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--mcis parameter value : ", mcisID)
			logger.Debug("--vm parameter value : ", vmID)

			SetupAndRun(cmd, args)
		},
	}

	vmDeleteCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	vmDeleteCmd.PersistentFlags().StringVarP(&mcisID, "mcis", "", "", "mcis id")
	vmDeleteCmd.PersistentFlags().StringVarP(&vmID, "vm", "", "", "mcis vm id")

	return vmDeleteCmd
}

// NewMcisVmStatusCmd - Mcis VM 상태 조회 기능을 수행하는 Cobra Command 생성
func NewMcisVmStatusCmd() *cobra.Command {

	vmStatusCmd := &cobra.Command{
		Use:   "status-vm",
		Short: "This is status-vm command for mcis",
		Long:  "This is status-vm command for mcis",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			if mcisID == "" {
				logger.Error("failed to validate --mcis parameter")
				return
			}
			if vmID == "" {
				logger.Error("failed to validate --vm parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--mcis parameter value : ", mcisID)
			logger.Debug("--vm parameter value : ", vmID)

			SetupAndRun(cmd, args)
		},
	}

	vmStatusCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	vmStatusCmd.PersistentFlags().StringVarP(&mcisID, "mcis", "", "", "mcis id")
	vmStatusCmd.PersistentFlags().StringVarP(&vmID, "vm", "", "", "mcis vm id")

	return vmStatusCmd
}

// NewMcisVmSuspendCmd - Mcis VM Suspend 기능을 수행하는 Cobra Command 생성
func NewMcisVmSuspendCmd() *cobra.Command {

	vmSuspendCmd := &cobra.Command{
		Use:   "suspend-vm",
		Short: "This is suspend-vm command for mcis",
		Long:  "This is suspend-vm command for mcis",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			if mcisID == "" {
				logger.Error("failed to validate --mcis parameter")
				return
			}
			if vmID == "" {
				logger.Error("failed to validate --vm parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--mcis parameter value : ", mcisID)
			logger.Debug("--vm parameter value : ", vmID)

			SetupAndRun(cmd, args)
		},
	}

	vmSuspendCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	vmSuspendCmd.PersistentFlags().StringVarP(&mcisID, "mcis", "", "", "mcis id")
	vmSuspendCmd.PersistentFlags().StringVarP(&vmID, "vm", "", "", "mcis vm id")

	return vmSuspendCmd
}

// NewMcisVmResumeCmd - Mcis VM Resume 기능을 수행하는 Cobra Command 생성
func NewMcisVmResumeCmd() *cobra.Command {

	vmResumeCmd := &cobra.Command{
		Use:   "resume-vm",
		Short: "This is resume-vm command for mcis",
		Long:  "This is resume-vm command for mcis",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			if mcisID == "" {
				logger.Error("failed to validate --mcis parameter")
				return
			}
			if vmID == "" {
				logger.Error("failed to validate --vm parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--mcis parameter value : ", mcisID)
			logger.Debug("--vm parameter value : ", vmID)

			SetupAndRun(cmd, args)
		},
	}

	vmResumeCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	vmResumeCmd.PersistentFlags().StringVarP(&mcisID, "mcis", "", "", "mcis id")
	vmResumeCmd.PersistentFlags().StringVarP(&vmID, "vm", "", "", "mcis vm id")

	return vmResumeCmd
}

// NewMcisVmRebootCmd - Mcis VM Reboot 기능을 수행하는 Cobra Command 생성
func NewMcisVmRebootCmd() *cobra.Command {

	vmRebootCmd := &cobra.Command{
		Use:   "reboot-vm",
		Short: "This is reboot-vm command for mcis",
		Long:  "This is reboot-vm command for mcis",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			if mcisID == "" {
				logger.Error("failed to validate --mcis parameter")
				return
			}
			if vmID == "" {
				logger.Error("failed to validate --vm parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--mcis parameter value : ", mcisID)
			logger.Debug("--vm parameter value : ", vmID)

			SetupAndRun(cmd, args)
		},
	}

	vmRebootCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	vmRebootCmd.PersistentFlags().StringVarP(&mcisID, "mcis", "", "", "mcis id")
	vmRebootCmd.PersistentFlags().StringVarP(&vmID, "vm", "", "", "mcis vm id")

	return vmRebootCmd
}

// NewMcisVmTerminateCmd - Mcis VM Terminate 기능을 수행하는 Cobra Command 생성
func NewMcisVmTerminateCmd() *cobra.Command {

	vmTerminateCmd := &cobra.Command{
		Use:   "terminate-vm",
		Short: "This is terminate-vm command for mcis",
		Long:  "This is terminate-vm command for mcis",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			if nameSpaceID == "" {
				logger.Error("failed to validate --ns parameter")
				return
			}
			if mcisID == "" {
				logger.Error("failed to validate --mcis parameter")
				return
			}
			if vmID == "" {
				logger.Error("failed to validate --vm parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--mcis parameter value : ", mcisID)
			logger.Debug("--vm parameter value : ", vmID)

			SetupAndRun(cmd, args)
		},
	}

	vmTerminateCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	vmTerminateCmd.PersistentFlags().StringVarP(&mcisID, "mcis", "", "", "mcis id")
	vmTerminateCmd.PersistentFlags().StringVarP(&vmID, "vm", "", "", "mcis vm id")

	return vmTerminateCmd
}

// NewCmdMcisCmd - MCIS 명령 실행 기능을 수행하는 Cobra Command 생성
func NewCmdMcisCmd() *cobra.Command {

	mcisCmdCmd := &cobra.Command{
		Use:   "command",
		Short: "This is execution command for mcis",
		Long:  "This is execution command for mcis",
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

	mcisCmdCmd.PersistentFlags().StringVarP(&inData, "indata", "d", "", "input string data")
	mcisCmdCmd.PersistentFlags().StringVarP(&inFile, "infile", "f", "", "input file path")

	return mcisCmdCmd
}

// NewCmdMcisVmCmd - MCIS VM 명령 실행 기능을 수행하는 Cobra Command 생성
func NewCmdMcisVmCmd() *cobra.Command {

	vmCmdCmd := &cobra.Command{
		Use:   "command-vm",
		Short: "This is command-vm command for mcis",
		Long:  "This is command-vm command for mcis",
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

	vmCmdCmd.PersistentFlags().StringVarP(&inData, "indata", "d", "", "input string data")
	vmCmdCmd.PersistentFlags().StringVarP(&inFile, "infile", "f", "", "input file path")

	return vmCmdCmd
}

// NewDeployMilkywayCmd - MCIS Agent 설치 기능을 수행하는 Cobra Command 생성
func NewDeployMilkywayCmd() *cobra.Command {

	deployMilkywayCmd := &cobra.Command{
		Use:   "deploy-milkyway",
		Short: "This is deploy-milkyway command for mcis",
		Long:  "This is deploy-milkyway command for mcis",
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

	deployMilkywayCmd.PersistentFlags().StringVarP(&inData, "indata", "d", "", "input string data")
	deployMilkywayCmd.PersistentFlags().StringVarP(&inFile, "infile", "f", "", "input file path")

	return deployMilkywayCmd
}

// NewAccessVmCmd - MCIS VM 에 SSH 접속 기능을 수행하는 Cobra Command 생성
func NewAccessVmCmd() *cobra.Command {

	accessVmCmd := &cobra.Command{
		Use:   "access-vm",
		Short: "This is access-vm command for mcis",
		Long:  "This is access-vm command for mcis",
		Run: func(cmd *cobra.Command, args []string) {
			SetupAndRun(cmd, args)
		},
	}

	return accessVmCmd
}

// NewBenchmarkCmd - MCIS VM 에 벤치마크 기능을 수행하는 Cobra Command 생성
func NewBenchmarkCmd() *cobra.Command {

	benchmarkCmd := &cobra.Command{
		Use:   "benchmark",
		Short: "This is benchmark command for mcis",
		Long:  "This is benchmark command for mcis",
		Run: func(cmd *cobra.Command, args []string) {
			SetupAndRun(cmd, args)
		},
	}

	return benchmarkCmd
}
