package config

import (
	"clx/constants/settings"
	"fmt"
	"github.com/spf13/viper"
	"os"
	"path"
	"runtime"
)

type Config struct {
	CommentWidth int `mapstructure:"CLX_COMMENT_WIDTH"`
	IndentSize   int `mapstructure:"CLX_INDENT_SIZE"`
}

func GetConfig() *Config {
	// Set the file name of the configurations file
	viper.SetConfigName(settings.ConfigName)

	cp := getConfigPath()
	viper.AddConfigPath(cp)

	//Check for environment variables
	viper.AutomaticEnv()

	viper.SetConfigType("env")

	configuration := new(Config)

	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Error reading config file, %s", err)
	}

	setDefaultValues()

	err := viper.Unmarshal(&configuration)
	if err != nil {
		fmt.Printf("Unable to decode into struct, %v", err)
	}

	return configuration
}

func setDefaultValues() {
	viper.SetDefault("CLX_COMMENT_WIDTH", "67")
	viper.SetDefault("CLX_INDENT_SIZE", "4")
}

func getConfigPath() string {
	clx := "circumflex"

	if runtime.GOOS == "linux" {
		config := os.Getenv("XDG_CONFIG_HOME")
		return path.Join(config, clx)
	}
	if runtime.GOOS == "darwin" {
		home := os.Getenv("HOME")
		config := ".config"

		return path.Join(home, config, clx)
	}

	return ""
}
