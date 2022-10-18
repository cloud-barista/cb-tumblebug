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

// NewMcisCmd : "cbadm mcis *" (for CB-Tumblebug)
func NewMcisCmd() *cobra.Command {

	mcisCmd := &cobra.Command{
		Use:   "mcis",
		Short: "This is a manageable command for mcis",
		Long:  "This is a manageable command for mcis",
	}

	//  Adds the commands for application.
	mcisCmd.AddCommand(NewMcisCreateCmd())
	mcisCmd.AddCommand(NewMcisListCmd())
	mcisCmd.AddCommand(NewMcisListIdCmd())
	mcisCmd.AddCommand(NewMcisGetCmd())
	mcisCmd.AddCommand(NewMcisDeleteCmd())
	mcisCmd.AddCommand(NewMcisDeleteAllCmd())
	mcisCmd.AddCommand(NewMcisStatusListCmd())
	mcisCmd.AddCommand(NewMcisStatusCmd())
	mcisCmd.AddCommand(NewMcisSuspendCmd())
	mcisCmd.AddCommand(NewMcisResumeCmd())
	mcisCmd.AddCommand(NewMcisRebootCmd())
	mcisCmd.AddCommand(NewMcisTerminateCmd())

	mcisCmd.AddCommand(NewMcisVmAddCmd())
	mcisCmd.AddCommand(NewMcisSubGroupCmd())
	mcisCmd.AddCommand(NewMcisVmListCmd())
	mcisCmd.AddCommand(NewMcisVmListIdCmd())
	mcisCmd.AddCommand(NewMcisVmGetCmd())
	mcisCmd.AddCommand(NewMcisVmDeleteCmd())
	mcisCmd.AddCommand(NewMcisVmStatusCmd())
	mcisCmd.AddCommand(NewMcisVmSuspendCmd())
	mcisCmd.AddCommand(NewMcisVmResumeCmd())
	mcisCmd.AddCommand(NewMcisVmRebootCmd())
	mcisCmd.AddCommand(NewMcisVmTerminateCmd())

	mcisCmd.AddCommand(NewMcisRecommendCmd())
	mcisCmd.AddCommand(NewMcisRecommendVmCmd())

	mcisCmd.AddCommand(NewCmdMcisCmd())
	mcisCmd.AddCommand(NewCmdMcisVmCmd())

	mcisCmd.AddCommand(NewDeployMilkywayCmd())

	mcisCmd.AddCommand(NewAccessVmCmd())
	mcisCmd.AddCommand(NewBenchmarkCmd())

	mcisCmd.AddCommand(NewInstallMonAgentCmd())
	mcisCmd.AddCommand(NewGetMonDataCmd())

	mcisCmd.AddCommand(NewMcisCreatePolicyCmd())
	mcisCmd.AddCommand(NewMcisListPolicyCmd())
	mcisCmd.AddCommand(NewMcisGetPolicyCmd())
	mcisCmd.AddCommand(NewMcisDeletePolicyCmd())
	mcisCmd.AddCommand(NewMcisDeleteAllPolicyCmd())

	return mcisCmd
}

// NewMcisCreateCmd : "cbadm mcis create"
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

// NewMcisListCmd : "cbadm mcis list"
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
			logger.Debug("--ns parameter value : ", nameSpaceID)

			SetupAndRun(cmd, args)
		},
	}

	listCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")

	return listCmd
}

// NewMcisListIdCmd : "cbadm mcis list-id"
func NewMcisListIdCmd() *cobra.Command {

	listIdCmd := &cobra.Command{
		Use:   "list-id",
		Short: "This is list-id command for mcis",
		Long:  "This is list-id command for mcis",
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

// NewMcisGetCmd : "cbadm mcis get"
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

// NewMcisDeleteCmd : "cbadm mcis delete"
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
			logger.Debug("--option parameter value : ", option)

			SetupAndRun(cmd, args)
		},
	}

	deleteCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	deleteCmd.PersistentFlags().StringVarP(&mcisID, "mcis", "", "", "mcis id")
	deleteCmd.PersistentFlags().StringVarP(&option, "option", "", "", "option [terminate, force]")

	return deleteCmd
}

// NewMcisDeleteAllCmd : "cbadm mcis delete-all"
func NewMcisDeleteAllCmd() *cobra.Command {

	deleteAllCmd := &cobra.Command{
		Use:   "delete-all",
		Short: "This is delete-all command for mcis",
		Long:  "This is delete-all command for mcis",
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

	deleteAllCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")

	return deleteAllCmd
}

// NewMcisStatusListCmd : "cbadm mcis status-list"
func NewMcisStatusListCmd() *cobra.Command {

	statusListCmd := &cobra.Command{
		Use:   "status-list",
		Short: "This is status-list command for mcis",
		Long:  "This is status-list command for mcis",
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

	statusListCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")

	return statusListCmd
}

// NewMcisStatusCmd : "cbadm mcis status"
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

// NewMcisSuspendCmd : "cbadm mcis suspend"
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

// NewMcisResumeCmd : "cbadm mcis resume"
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

// NewMcisRebootCmd : "cbadm mcis reboot"
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

// NewMcisTerminateCmd : "cbadm mcis terminate"
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

// NewMcisVmAddCmd : "cbadm mcis add-vm"
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

// NewMcisSubGroupCmd : "cbadm mcis group-vm"
func NewMcisSubGroupCmd() *cobra.Command {

	subGroupCmd := &cobra.Command{
		Use:   "group-vm",
		Short: "This is group-vm command for mcis",
		Long:  "This is group-vm command for mcis",
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

	subGroupCmd.PersistentFlags().StringVarP(&inData, "indata", "d", "", "input string data")
	subGroupCmd.PersistentFlags().StringVarP(&inFile, "infile", "f", "", "input file path")

	return subGroupCmd
}

// NewMcisVmListCmd : "cbadm mcis list-vm"
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

// NewMcisVmListIdCmd : "cbadm mcis list-vm-id"
func NewMcisVmListIdCmd() *cobra.Command {

	vmListIdCmd := &cobra.Command{
		Use:   "list-vm-id",
		Short: "This is list-vm-id command for mcis",
		Long:  "This is list-vm-id command for mcis",
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

	vmListIdCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	vmListIdCmd.PersistentFlags().StringVarP(&mcisID, "mcis", "", "", "mcis id")

	return vmListIdCmd
}

// NewMcisVmGetCmd : "cbadm mcis get-vm"
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

// NewMcisVmDeleteCmd : "cbadm mcis del-vm"
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

// NewMcisVmStatusCmd : "cbadm mcis status-vm"
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

// NewMcisVmSuspendCmd : "cbadm mcis suspend-vm"
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

// NewMcisVmResumeCmd : "cbadm mcis resume-vm"
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

// NewMcisVmRebootCmd : "cbadm mcis reboot-vm"
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

// NewMcisVmTerminateCmd : "cbadm mcis terminate-vm"
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

// NewMcisRecommendCmd : "cbadm mcis recommend"
func NewMcisRecommendCmd() *cobra.Command {

	recommendCmd := &cobra.Command{
		Use:   "recommend",
		Short: "This is recommend command for mcis",
		Long:  "This is recommend command for mcis",
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

	recommendCmd.PersistentFlags().StringVarP(&inData, "indata", "d", "", "input string data")
	recommendCmd.PersistentFlags().StringVarP(&inFile, "infile", "f", "", "input file path")

	return recommendCmd
}

// NewMcisRecommendVmCmd : "cbadm mcis recommend-vm"
func NewMcisRecommendVmCmd() *cobra.Command {

	recommendVmCmd := &cobra.Command{
		Use:   "recommend-vm",
		Short: "This is recommend-vm command for mcis",
		Long:  "This is recommend-vm command for mcis",
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

	recommendVmCmd.PersistentFlags().StringVarP(&inData, "indata", "d", "", "input string data")
	recommendVmCmd.PersistentFlags().StringVarP(&inFile, "infile", "f", "", "input file path")

	return recommendVmCmd
}

// NewCmdMcisCmd : "cbadm mcis command"
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

// NewCmdMcisVmCmd : "cbadm mcis command-vm"
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

// NewDeployMilkywayCmd : "cbadm mcis deploy-milkyway"
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

// NewAccessVmCmd : "cbadm mcis access-vm"
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

// NewBenchmarkCmd : "cbadm mcis benchmark"
func NewBenchmarkCmd() *cobra.Command {

	benchmarkCmd := &cobra.Command{
		Use:   "benchmark",
		Short: "This is benchmark command for mcis",
		Long:  "This is benchmark command for mcis",
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
			logger.Debug("--action parameter value : ", action)
			logger.Debug("--host parameter value : ", host)

			SetupAndRun(cmd, args)
		},
	}

	benchmarkCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	benchmarkCmd.PersistentFlags().StringVarP(&mcisID, "mcis", "", "", "mcis id")
	benchmarkCmd.PersistentFlags().StringVarP(&action, "action", "", "all", "action name")
	benchmarkCmd.PersistentFlags().StringVarP(&host, "host", "", "localhost", "target host ip address")

	return benchmarkCmd
}

// NewInstallMonAgentCmd : "cbadm mcis install-mon"
func NewInstallMonAgentCmd() *cobra.Command {

	installMonCmd := &cobra.Command{
		Use:   "install-mon",
		Short: "This is install-mon command for mcis",
		Long:  "This is install-mon command for mcis",
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

	installMonCmd.PersistentFlags().StringVarP(&inData, "indata", "d", "", "input string data")
	installMonCmd.PersistentFlags().StringVarP(&inFile, "infile", "f", "", "input file path")

	return installMonCmd
}

// NewGetMonDataCmd : "cbadm mcis get-mon"
func NewGetMonDataCmd() *cobra.Command {

	getMonCmd := &cobra.Command{
		Use:   "get-mon",
		Short: "This is get-mon command for mcis",
		Long:  "This is get-mon command for mcis",
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
			if metric == "" {
				logger.Error("failed to validate --metric parameter")
				return
			}
			logger.Debug("--ns parameter value : ", nameSpaceID)
			logger.Debug("--mcis parameter value : ", mcisID)
			logger.Debug("--metric parameter value : ", metric)

			SetupAndRun(cmd, args)
		},
	}

	getMonCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	getMonCmd.PersistentFlags().StringVarP(&mcisID, "mcis", "", "", "mcis id")
	getMonCmd.PersistentFlags().StringVarP(&metric, "metric", "", "", "metric")

	return getMonCmd
}

// NewMcisCreatePolicyCmd : "cbadm mcis create-policy"
func NewMcisCreatePolicyCmd() *cobra.Command {

	createPolicyCmd := &cobra.Command{
		Use:   "create-policy",
		Short: "This is create-policy command for mcis",
		Long:  "This is create-policy command for mcis",
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

	createPolicyCmd.PersistentFlags().StringVarP(&inData, "indata", "d", "", "input string data")
	createPolicyCmd.PersistentFlags().StringVarP(&inFile, "infile", "f", "", "input file path")

	return createPolicyCmd
}

// NewMcisListPolicyCmd : "cbadm mcis list-policy"
func NewMcisListPolicyCmd() *cobra.Command {

	listPolicyCmd := &cobra.Command{
		Use:   "list-policy",
		Short: "This is list-policy command for mcis",
		Long:  "This is list-policy command for mcis",
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

	listPolicyCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")

	return listPolicyCmd
}

// NewMcisGetPolicyCmd : "cbadm mcis get-policy"
func NewMcisGetPolicyCmd() *cobra.Command {

	getPolicyCmd := &cobra.Command{
		Use:   "get-policy",
		Short: "This is get-policy command for mcis",
		Long:  "This is get-policy command for mcis",
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

	getPolicyCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	getPolicyCmd.PersistentFlags().StringVarP(&mcisID, "mcis", "", "", "mcis id")

	return getPolicyCmd
}

// NewMcisDeletePolicyCmd : "cbadm mcis delete-policy"
func NewMcisDeletePolicyCmd() *cobra.Command {

	deletePolicyCmd := &cobra.Command{
		Use:   "delete-policy",
		Short: "This is delete-policy command for mcis",
		Long:  "This is delete-policy command for mcis",
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

	deletePolicyCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")
	deletePolicyCmd.PersistentFlags().StringVarP(&mcisID, "mcis", "", "", "mcis id")

	return deletePolicyCmd
}

// NewMcisDeleteAllPolicyCmd : "cbadm mcis delete-all-policy"
func NewMcisDeleteAllPolicyCmd() *cobra.Command {

	deleteAllPolicyCmd := &cobra.Command{
		Use:   "delete-all-policy",
		Short: "This is delete-all-policy command for mcis",
		Long:  "This is delete-all-policy command for mcis",
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

	deleteAllPolicyCmd.PersistentFlags().StringVarP(&nameSpaceID, "ns", "", "", "namespace id")

	return deleteAllPolicyCmd
}
