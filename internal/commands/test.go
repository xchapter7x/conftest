package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/open-policy-agent/conftest/internal/runner"
	"github.com/open-policy-agent/conftest/output"
	"github.com/open-policy-agent/conftest/parser"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const testDesc = `
This command tests your configuration files using the Open Policy Agent.

The test command expects one or more input files that will be evaluated 
against Open Policy Agent policies. Directories are also supported as valid
inputs. 

Policies are written in the Rego language. For more
information on how to write Rego policies, see the documentation:
https://www.openpolicyagent.org/docs/latest/policy-language/

The policy location defaults to the policy directory in the local folder.
The location can be overridden with the '--policy' flag, e.g.:

	$ conftest test --policy <my-directory> <input-file(s)/input-folder>

Some policies are dependant on external data. This data is loaded in seperatly 
from policies. The location of any data directory or file can be specified with 
the '--data' flag. If a directory is specified, it will be recursively searched for 
any data files. Right now any '.json' or '.yaml' file will be loaded in 
and made available in the Rego policies. Data will be made available in Rego based on 
the file path where the data was found. For example, if data is stored 
under 'policy/exceptions/my_data.yaml', and we execute the following command:

	$ conftest test --data policy <input-file>

The data is available under 'import data.exceptions'.

The test command supports the '--output' flag to specify the type, e.g.:

	$ conftest test -o table -p examples/kubernetes/policy examples/kubernetes/deployment.yaml

Which will return the following output:
+---------+----------------------------------+--------------------------------+
| RESULT  |               FILE               |            MESSAGE             |
+---------+----------------------------------+--------------------------------+
| success | examples/kubernetes/service.yaml |                                |
| warning | examples/kubernetes/service.yaml | Found service hello-kubernetes |
|         |                                  | but services are not allowed   |
+---------+----------------------------------+--------------------------------+

By default, it will use the regular stdout output. For a full list of available output types, see the of the '--output' flag.

The test command supports the '--update' flag to fetch the latest version of the policy at the given url.
It expects one or more urls to fetch the latest policies from, e.g.:

	$ conftest test --update instrumenta.azurecr.io/test

See the pull command for more details on supported protocols for fetching policies.

When debugging policies it can be useful to use a more verbose policy evaluation output. By using the '--trace' flag
the output will include a detailed trace of how the policy was evaluated, e.g.

	$ conftest test --trace <input-file>
`

// TestRun stores the compiler and store for a test run.
type TestRun struct {
	Compiler *ast.Compiler
	Store    storage.Store
}

// NewTestCommand creates a new test command.
func NewTestCommand(ctx context.Context) *cobra.Command {
	cmd := cobra.Command{
		Use:   "test <file> [file...]",
		Short: "Test your configuration files using Open Policy Agent",
		Long:  testDesc,
		Args:  cobra.MinimumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			flagNames := []string{"all-namespaces", "combine", "data", "fail-on-warn", "ignore", "input", "namespace", "no-color", "output", "policy", "trace", "update"}
			for _, name := range flagNames {
				if err := viper.BindPFlag(name, cmd.Flags().Lookup(name)); err != nil {
					return fmt.Errorf("bind flag: %w", err)
				}
			}

			return nil
		},

		RunE: func(cmd *cobra.Command, fileList []string) error {
			var runner runner.TestRunner
			if err := viper.Unmarshal(&runner); err != nil {
				return fmt.Errorf("unmarshal parameters: %w", err)
			}

			results, err := runner.Run(ctx, fileList)
			if err != nil {
				return fmt.Errorf("running test: %w", err)
			}

			outputManager := output.GetOutputManager(runner.Output, !runner.NoColor)
			if runner.Trace {
				outputManager = outputManager.WithTracing()
			}

			for _, result := range results {
				if err := outputManager.Put(result); err != nil {
					return fmt.Errorf("put: %w", err)
				}
			}

			if err := outputManager.Flush(); err != nil {
				return fmt.Errorf("flushing output: %w", err)
			}

			var exitCode int
			if runner.FailOnWarn {
				exitCode = output.ExitCodeFailOnWarn(results)
			} else {
				exitCode = output.ExitCode(results)
			}
			if exitCode > 0 {
				os.Exit(exitCode)
			}

			return nil
		},
	}

	cmd.Flags().Bool("fail-on-warn", false, "Return a non-zero exit code if warnings or errors are found")
	cmd.Flags().BoolP("trace", "", false, "Enable more verbose trace output for Rego queries")
	cmd.Flags().Bool("no-color", false, "Disable color when printing")
	cmd.Flags().Bool("all-namespaces", false, "Test policies found in all namespaces")
	cmd.Flags().BoolP("combine", "", false, "Combine all config files to be evaluated together")

	cmd.Flags().String("ignore", "", "A regex pattern which can be used for ignoring paths")
	cmd.Flags().StringP("output", "o", "", fmt.Sprintf("Output format for conftest results - valid options are: %s", output.ValidOutputs()))
	cmd.Flags().StringP("input", "i", "", fmt.Sprintf("Input type for given source, especially useful when using conftest with stdin, valid options are: %s", parser.ValidInputs()))

	cmd.Flags().StringSliceP("policy", "p", []string{"policy"}, "Path to the Rego policy files directory")
	cmd.Flags().StringSliceP("update", "u", []string{}, "A list of URLs can be provided to the update flag, which will download before the tests run")
	cmd.Flags().StringSliceP("namespace", "n", []string{"main"}, "Test policies in a specific namespace")
	cmd.Flags().StringSliceP("data", "d", []string{}, "A list of paths from which data for the rego policies will be recursively loaded")

	return &cmd
}
