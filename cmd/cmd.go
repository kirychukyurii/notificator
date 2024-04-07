package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/webitel/wlog"

	"github.com/kirychukyurii/notificator/config"
)

var (

	// version is the App's semantic version.
	version = "0.0.0"

	// commit is the git commit used to build the App.
	commit     = "hash"
	commitDate = "date"

	configPath = "config.yaml"
)

func Execute() {
	if err := command().Execute(); err != nil {
		os.Exit(-1)
	}
}

func command() *cobra.Command {
	log := wlog.NewLogger(&wlog.LoggerConfiguration{
		EnableConsole: true,
		ConsoleLevel:  "debug",
	})

	cfg, err := config.New(configPath)
	if err != nil {
		log.Critical("parsing config", wlog.Err(err))
	}

	c := &cobra.Command{
		Use:          "notificator",
		Short:        "Notificator - simplifier notifications delivery",
		SilenceUsage: true,
		Version:      fmt.Sprintf("%s, commit %s, date %s", version, commit, commitDate),
		Args: func(cmd *cobra.Command, args []string) error {

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// apply CLI args to config
			if err := cmd.ParseFlags(os.Args[1:]); err != nil {
				return fmt.Errorf("parsing flags: %w", err)
			}

			return nil
		},
	}

	flagSet(c.PersistentFlags())
	c.AddCommand(listenCommand(cfg, log))

	return c
}

func flagSet(fs *pflag.FlagSet) {
	fs.StringVarP(&configPath, "config", "c", "", "config file path")
}
