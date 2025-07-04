package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/urfave/cli/v3"
)

// Config holds the configuration for the gdocker.
// It contains the path to the Docker binary and the directory where Docker images are stored.
type Config struct {
	DockerBin string `json:"docker_bin"`
	Dir       string `json:"dir"`
	StockDir  string `json:"stock_dir,omitempty"` // Optional field for stock directory
}

// NewConfig creates a new Config instance.
func NewConfig(dockerBin, dir string) *Config {
	return &Config{
		DockerBin: dockerBin,
		Dir:       dir,
	}
}

// update* functions are used to update the configuration fields.
// They check if the new value is different from the current value and update it if necessary.
// They also log the change if it occurs.
// They return true if the value was updated, false otherwise.
func (c *Config) updateDockerBin(docker_bin string) bool {
	if docker_bin != "" && c.DockerBin != docker_bin {
		slog.Info(fmt.Sprintf("config docker_bin '%s' is overridden by '%s'", c.DockerBin, docker_bin))
		c.DockerBin = docker_bin
		return true
	}
	return false
}

func (c *Config) updateDir(dir string) bool {
	if dir != "" && c.Dir != dir {
		slog.Info(fmt.Sprintf("config dir '%s' is overridden by '%s'", c.Dir, dir))
		c.Dir = dir
		return true
	}
	return false
}

func (c *Config) updateStockDir(stock string) bool {
	if stock != "" && c.StockDir != stock {
		slog.Info(fmt.Sprintf("config stock '%s' is overridden by '%s'", c.StockDir, stock))
		c.StockDir = stock
		return true
	}
	return false
}

// loadAndSaveConfig loads the configuration from a file or creates a new one if it doesn't exist.
// It updates the configuration with command line arguments if they are set.
// If the configuration is updated, it writes the new configuration to the file.
// It returns the final configuration.
func loadAndSaveConfig(cmd *cli.Command) Config {
	var config Config
	var err error
	write := false
	file := searchConfigFiles(cmd.StringSlice("config"))
	if isFile(file) {
		config, err = readConfig(file)
		if err != nil {
			slog.Error(err.Error())
			os.Exit(1)
		}
		printConfig(config)
	} else {
		config = *NewConfig(
			cmd.String("docker-bin"),
			cmd.String("dir"),
		)
		write = true
	}

	if cmd.IsSet("docker-bin") && config.updateDockerBin(cmd.String("docker-bin")) {
		write = true
	}
	if cmd.IsSet("dir") && config.updateDir(cmd.String("dir")) {
		write = true
	}
	if cmd.IsSet("stock") && config.updateStockDir(cmd.String("stock")) {
		write = true
	}
	if write {
		if config.DockerBin == "" || config.Dir == "" {
			slog.Error("docker-bin and dir must be set")
			os.Exit(1)
		}
		writeConfig(file, config, cmd.Bool("dry-run"))
		printConfig(config)
	}

	return config
}

// loadConfig loads the configuration from a file and updates it with command line arguments if they are set.
// It returns the final configuration.
// It does not write the configuration to a file.
func loadConfig(cmd *cli.Command) Config {
	var config Config
	var err error
	file := searchConfigFiles(cmd.StringSlice("config"))
	config, err = readConfig(file)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	printConfig(config)

	if cmd.IsSet("docker-bin") {
		config.updateDockerBin(cmd.String("docker-bin"))
	}
	if cmd.IsSet("dir") {
		config.updateDir(cmd.String("dir"))
	}
	if config.StockDir == "" || cmd.IsSet("stock") {
		config.updateStockDir(cmd.String("stock"))
	}

	return config
}

// searchConfigFiles searches for configuration files in the provided list of files.
// It returns the last file that exists, or the first file in the list if none exist.
func searchConfigFiles(files []string) string {
	var final string
	for _, file := range files {
		if isFile(file) {
			final = file
		}
	}
	if final == "" {
		return files[0]
	}
	return final
}

func printConfig(config Config) {
	fmt.Printf("Docker binary: %s\n", config.DockerBin)
	fmt.Printf("Docker image directory: %s\n", config.Dir)
}

// readConfig reads the configuration from a JSON file.
// It returns the configuration and an error if any occurs.
func readConfig(file string) (Config, error) {
	var config Config
	f, err := os.Open(file)
	if err != nil {
		return config, err
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	if err := decoder.Decode(&config); err != nil {
		return config, err
	}
	slog.Info(fmt.Sprintf("configuration was read from '%s'", file))
	return config, err
}

// writeConfig writes the configuration to a JSON file.
// If dry_run is true, it only show a log.
func writeConfig(file string, config Config, dry_run bool) {
	if !dry_run {
		var w io.Writer
		if file == "stdout" {
			w = os.Stdout
		} else {
			var f *os.File
			f, err := os.Create(file)
			if err != nil {
				slog.Error(err.Error())
				os.Exit(1)
			}
			defer f.Close()
			w = f
		}
		b, err := json.Marshal(config)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		w.Write(b)
	}
	slog.Info(fmt.Sprintf("configuration was write to '%s'", file))
}
