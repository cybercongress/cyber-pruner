package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	homePath     string
	dataDir      = "data"
	configDir    = "config/app.toml"
	backend      string
	cosmosSdk    bool
	tendermint   bool
	txIndex      bool
	blocks       uint64
	keepVersions uint64
	keepEvery    uint64
	batch        uint64
	parallel     uint64
	profile      string
	compact      bool
	appName      = "cyber-pruner"
)

func cobraInit(rootCmd *cobra.Command) error {
	if homePath == "" {
		dirname, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		homePath = rootify(".cyber", dirname)
	}

	appDir := rootify(configDir, homePath)
	// Use config file from the flag.
	viper.SetConfigFile(appDir)

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("Error loading config file. %+v", err)
	}
	if viper.ConfigFileUsed() != "" {
		fmt.Printf("\nUsing config file: %+v ", viper.ConfigFileUsed())
	}
	// Bind flags from the command line to the viper framework
	if err := viper.BindPFlags(rootCmd.Flags()); err != nil {
		return err
	}

	blocks = viper.GetUint64("min-retain-blocks")
	profile = viper.GetString("pruning")
	keepVersions = viper.GetUint64("pruning-keep-recent")
	keepEvery = viper.GetUint64("pruning-keep-every")

	return nil
}

// NewRootCmd returns the root command for relayer.
func NewRootCmd() *cobra.Command {
	// RootCmd represents the base command when called without any subcommands
	var rootCmd = &cobra.Command{
		Use:   appName,
		Short: "cyber-pruner is meant to prune data base history from a cosmos application, avoiding needing to state sync every couple amount of weeks",
	}

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		// reads `homeDir/config.yaml` into `var config *Config` before each command
		if err := cobraInit(rootCmd); err != nil {
			return err
		}

		return nil
	}

	// --home flag
	rootCmd.PersistentFlags().StringVar(&homePath, "home", "", `directory for config and data (""=default /.cyber directory) (default "")`)
	if err := viper.BindPFlag("home", rootCmd.PersistentFlags().Lookup("home")); err != nil {
		panic(err)
	}

	// --pruning flag
	rootCmd.PersistentFlags().StringVar(&profile, "pruning", "default", "pruning profile")
	if err := viper.BindPFlag("pruning", rootCmd.PersistentFlags().Lookup("pruning")); err != nil {
		panic(err)
	}

	// --min-retain-blocks flag
	rootCmd.PersistentFlags().
		Uint64Var(&blocks, "min-retain-blocks", 0, "set the amount of tendermint blocks to be kept (0=keep all) (default 0)")
	if err := viper.BindPFlag("min-retain-blocks", rootCmd.PersistentFlags().Lookup("min-retain-blocks")); err != nil {
		panic(err)
	}

	// --pruning-keep-recent flag
	rootCmd.PersistentFlags().
		Uint64Var(&keepVersions, "pruning-keep-recent", 400000, "set the amount of versions to keep in the application store")
	if err := viper.BindPFlag("pruning-keep-recent", rootCmd.PersistentFlags().Lookup("pruning-keep-recent")); err != nil {
		panic(err)
	}

	// --pruning-keep-every flag
	rootCmd.PersistentFlags().
		Uint64Var(&keepEvery, "pruning-keep-every", 1000, "set the version interval to be kept in the application store (0=None)")
	if err := viper.BindPFlag("pruning-keep-every", rootCmd.PersistentFlags().Lookup("pruning-keep-every")); err != nil {
		panic(err)
	}

	// --batch flag
	rootCmd.PersistentFlags().
		Uint64Var(&batch, "batch", 10000, "set the amount of versions to be pruned in one batch")
	if err := viper.BindPFlag("batch", rootCmd.PersistentFlags().Lookup("batch")); err != nil {
		panic(err)
	}

	// --parallel-limit flag
	rootCmd.PersistentFlags().
		Uint64Var(&parallel, "parallel-limit", 16, "set the limit of parallel go routines to be running at the same time")
	if err := viper.BindPFlag("parallel-limit", rootCmd.PersistentFlags().Lookup("parallel-limit")); err != nil {
		panic(err)
	}

	// --backend flag
	rootCmd.PersistentFlags().
		StringVar(&backend, "backend", "goleveldb", "set the type of db being used")
	// todo add list of dbs to comment
	if err := viper.BindPFlag("backend", rootCmd.PersistentFlags().Lookup("backend")); err != nil {
		panic(err)
	}

	// --cosmos-sdk flag
	rootCmd.PersistentFlags().
		BoolVar(&cosmosSdk, "cosmos-sdk", true, "set to false if using only with tendermint (default true)")
	if err := viper.BindPFlag("cosmos-sdk", rootCmd.PersistentFlags().Lookup("cosmos-sdk")); err != nil {
		panic(err)
	}

	// --tendermint flag
	rootCmd.PersistentFlags().
		BoolVar(&tendermint, "tendermint", true, "set to false you dont want to prune tendermint data (default true)")
	if err := viper.BindPFlag("tendermint", rootCmd.PersistentFlags().Lookup("tendermint")); err != nil {
		panic(err)
	}

	// --tx-idx flag
	rootCmd.PersistentFlags().
		BoolVar(&txIndex, "tx-index", false, "set to false you dont want to prune transactions and blocks events (default false)")
	if err := viper.BindPFlag("tx-index", rootCmd.PersistentFlags().Lookup("tx-index")); err != nil {
		panic(err)
	}

	rootCmd.PersistentFlags().BoolVar(&compact, "compact", true, "set to false you dont want to compact dbs after prunning (default true)")
	if err := viper.BindPFlag("compact", rootCmd.PersistentFlags().Lookup("compact")); err != nil {
		panic(err)
	}

	rootCmd.AddCommand(
		pruneCmd(),
		compactCmd(),
	)

	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.EnableCommandSorting = false

	rootCmd := NewRootCmd()
	rootCmd.SilenceUsage = true
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
