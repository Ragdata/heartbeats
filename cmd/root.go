/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gi8lino/heartbeats/internal"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	version = "0.0.1"
)

var debug bool

type PlainFormatter struct{}

func (f *PlainFormatter) Format(entry *log.Entry) ([]byte, error) {
	return []byte(fmt.Sprintf("%s %s\n", entry.Time.Format(time.RFC3339), entry.Message)), nil
}
func toggleDebug(cmd *cobra.Command, args []string) {

	plainFormatter := new(PlainFormatter)
	log.SetFormatter(plainFormatter)

	if debug {
		internal.HeartbeatsServer.Config.Debug = true
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug logs enabled")
	}
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "heartbeat",
	Short: "Wait for heartbeats and notify if they are missing",
	Long: `Heartbeats waits for heartbeats and notifies if they are missing.
You can configure the interval and grace period for each heartbeat separately and it will notify you if a heartbeat is missing.
`,

	PersistentPreRun: toggleDebug,
	Run: func(cmd *cobra.Command, args []string) {

		if internal.HeartbeatsServer.Config.PrintVersion {
			fmt.Println(version)
			os.Exit(0)
		}

		internal.HeartbeatsServer.Version = version

		// Run the server
		internal.HeartbeatsServer.Run()
	},
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.Flags().StringVarP(&internal.HeartbeatsServer.Config.Path, "config", "c", "./config.yaml", "path to notifications config file")

	rootCmd.Flags().BoolVarP(&debug, "debug", "d", false, "Verbose logging.")
	rootCmd.Flags().BoolVarP(&internal.HeartbeatsServer.Config.PrintVersion, "version", "v", false, "Print the current version and exit.")
	rootCmd.Flags().StringVar(&internal.HeartbeatsServer.Server.Hostname, "host", "127.0.0.1", "Host of Heartbeat service.")
	rootCmd.Flags().IntVarP(&internal.HeartbeatsServer.Server.Port, "port", "p", 8090, "Port to listen on")
}

func initConfig() {
	if err := internal.ReadConfigFile(internal.HeartbeatsServer.Config.Path); err != nil {
		log.Fatal(err)
	}

	viper.New().OnConfigChange(func(e fsnotify.Event) {
		log.Info("config file changed:", e.Name)
		if err := internal.ReadConfigFile(internal.HeartbeatsServer.Config.Path); err != nil {
			log.Fatal(err)
		}
	})
	viper.WatchConfig()
}
