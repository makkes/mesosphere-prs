package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

type Config struct {
	User      string
	Token     string
	Teammates []string
}

func obtainConfigurationInteractively(config *Config) error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Please enter your GitHub username: ")
	text, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	config.User = strings.Trim(text, "\n")
	fmt.Printf("Please enter a GitHub personal access token: ")
	text, err = reader.ReadString('\n')
	if err != nil {
		return err
	}
	config.Token = strings.Trim(text, "\n")
	fmt.Printf("Please enter all GitHub usernames of your teammates (separated with commas): ")
	text, err = reader.ReadString('\n')
	if err != nil {
		return err
	}
	config.Teammates = strings.Split(regexp.MustCompile(`\s*`).ReplaceAllString(text, ""), ",")
	return nil
}

func writeConfigFile(f *os.File, config Config) error {
	bytes, err := json.Marshal(config)
	if err != nil {
		return err
	}
	_, err = f.Write(bytes)
	if err != nil {
		return err
	}
	return nil
}

func sanityCheck(config Config) error {
	invalidFields := make([]string, 0)
	if config.User == "" {
		invalidFields = append(invalidFields, "user must not be empty")
	}
	if config.Token == "" {
		invalidFields = append(invalidFields, "token must not be empty")
	}
	if len(invalidFields) > 0 {
		return fmt.Errorf("configuration invalid: %s", strings.Join(invalidFields, ", "))
	}
	return nil
}

func Init() (*Config, error) {
	configFileLocation := path.Join(os.Getenv("HOME"), ".prs.json")
	configFile, err := os.OpenFile(configFileLocation, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	defer configFile.Close()
	configBytes, err := ioutil.ReadAll(configFile)
	if err != nil {
		return nil, err
	}
	config := Config{}
	if len(configBytes) > 0 {
		err = json.Unmarshal(configBytes, &config)
		if err != nil {
			return nil, err
		}
	} else {
		err = obtainConfigurationInteractively(&config)
		if err != nil {
			return nil, err
		}
		err = writeConfigFile(configFile, config)
		if err != nil {
			return nil, err
		}
	}

	err = sanityCheck(config)
	if err != nil {
		return nil, errors.Wrapf(err, "please check your configuration file %s", configFileLocation)
	}
	return &config, nil
}
