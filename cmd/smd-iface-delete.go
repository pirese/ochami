// This source code is licensed under the license found in the LICENSE file at
// the root directory of this source tree.
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/OpenCHAMI/ochami/internal/log"
	"github.com/OpenCHAMI/ochami/pkg/client"
	"github.com/OpenCHAMI/ochami/pkg/client/smd"
)

// ifaceDeleteCmd represents the "smd iface delete" command
var ifaceDeleteCmd = &cobra.Command{
	Use:   "delete (-d (<payload_data> | @<payload_file>)) | --all | <iface_id>...",
	Short: "Delete one or more ethernet interfaces",
	Long: `Delete one or more ethernet interfaces. These can be specified by one
or more ethernet interface IDs (note this is not the same as a
component xname). Alternatively, pass -d to pass raw payload data
or (if flag argument starts with @) a file containing the
payload data. -f can be specified to change the format of
the input payload data ('json' by default), but the rules
above still apply for the payload. If "-" is used as the
input payload filename, the data is read from standard input.

This command sends a DELETE to SMD. An access token is required.

See ochami-smd(1) for more details.`,
	Example: `  # Delete ethernet interface using CLI flags
  ochami smd iface delete decafc0ffeee
  ochami smd iface delete decafc0ffeee de:ad:be:ee:ee:ef
  ochami smd iface delete --all

  # Delete ethernet interfaces using input payload file
  ochami smd iface delete -d @payload.json
  ochami smd iface delete -d @payload.yaml -f yaml

  # Delete ethernet interfaces using data from standard input
  echo '<json_data>' | ochami smd iface delete -d @-
  echo '<yaml_data>' | ochami smd iface delete -d @- -f yaml`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// With options, only one of:
		// - A payload file with -d
		// - --all
		// - A set of one or more ethernet interface IDs
		// must be passed.
		if !cmd.Flag("all").Changed && !cmd.Flag("data").Changed {
			if len(args) == 0 {
				return fmt.Errorf("expected -d, --all, or >= 1 argument (interface id), got %d", len(args))
			}
		} else {
			if len(args) > 0 {
				log.Logger.Warn().Msgf("raw data or --all passed, ignoring extra arguments: %v", args)
			}
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Create client to use for requests
		smdClient := smdGetClient(cmd)

		// Handle token for this command
		handleToken(cmd)

		// Ask before attempting deletion unless --no-confirm was passed
		if !cmd.Flag("no-confirm").Changed {
			log.Logger.Debug().Msg("--no-confirm not passed, prompting user to confirm deletion")
			var respDelete bool
			var err error
			if cmd.Flag("all").Changed {
				respDelete, err = ios.loopYesNo("Really delete ALL ETHERNET INTERFACES?")
			} else {
				respDelete, err = ios.loopYesNo("Really delete?")
			}
			if err != nil {
				log.Logger.Error().Err(err).Msg("Error fetching user input")
				os.Exit(1)
			} else if !respDelete {
				log.Logger.Info().Msg("User aborted ethernet interface deletion")
				os.Exit(0)
			} else {
				log.Logger.Debug().Msg("User answered affirmatively to delete ethernet interfaces")
			}
		}

		// Create list of ethernet interface IDs to delete
		var eiSlice []smd.EthernetInterface
		var eIdSlice []string
		if cmd.Flag("data").Changed {
			// Use payload file if passed
			handlePayload(cmd, &eiSlice)
		} else {
			// ...otherwise, use passed CLI arguments
			eIdSlice = args
		}

		// Perform deletion
		if cmd.Flag("all").Changed {
			// If --all passed, we don't care about any passed arguments
			_, err := smdClient.DeleteEthernetInterfacesAll(token)
			if err != nil {
				if errors.Is(err, client.UnsuccessfulHTTPError) {
					log.Logger.Error().Err(err).Msg("SMD ethernet interface deletion yielded unsuccessful HTTP response")
				} else {
					log.Logger.Error().Err(err).Msg("failed to delete ethernet interfaces in SMD")
				}
				logHelpError(cmd)
				os.Exit(1)
			}
		} else {
			// If --all not passed, pass argument list to deletion logic
			_, errs, err := smdClient.DeleteEthernetInterfaces(token, eIdSlice...)
			if err != nil {
				log.Logger.Error().Err(err).Msg("failed to delete ethernet interfaces in SMD")
				logHelpError(cmd)
				os.Exit(1)
			}
			// Since smdClient.DeleteEthernetInterfaces does the deletion iteratively, we need to deal
			// with each error that might have occurred.
			var errorsOccurred = false
			for _, e := range errs {
				if err != nil {
					if errors.Is(e, client.UnsuccessfulHTTPError) {
						log.Logger.Error().Err(e).Msg("SMD ethernet interface deletion yielded unsuccessful HTTP response")
					} else {
						log.Logger.Error().Err(e).Msg("failed to delete ethernet interfaces")
					}
					errorsOccurred = true
				}
			}
			// Warn the user if any errors occurred during deletion iterations
			if errorsOccurred {
				log.Logger.Warn().Msg("SMD ethernet interface deletion completed with errors")
				logHelpError(cmd)
				os.Exit(1)
			}
		}
	},
}

func init() {
	ifaceDeleteCmd.Flags().BoolP("all", "a", false, "delete all ethernet interfaces in SMD")
	ifaceDeleteCmd.Flags().StringP("data", "d", "", "payload data or (if starting with @) file containing payload data (can be - to read from stdin)")
	ifaceDeleteCmd.Flags().VarP(&formatInput, "format-input", "f", "format of input payload data (json,json-pretty,yaml)")
	ifaceDeleteCmd.Flags().Bool("no-confirm", false, "do not ask before attempting deletion")

	ifaceDeleteCmd.RegisterFlagCompletionFunc("format-input", completionFormatData)

	ifaceCmd.AddCommand(ifaceDeleteCmd)
}
