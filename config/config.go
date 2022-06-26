package config

import (
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"time"
)

type OauthSecret struct {
	Id     string `json:"id"`
	Secret string `json:"secret"`
}

type Token struct {
	AccessToken  string    `yaml:"access_token"`
	TokenType    string    `yaml:"token_type"`
	ExpiresAt    time.Time `yaml:"expires_at"`
	RefreshToken string    `yaml:"refresh_token"`
	IdToken      string    `yaml:"id_token"`
}

type Config struct {
	Hostname      string `yaml:"hostname"`
	DebugHostname string `yaml:"debug_hostname"`
	Upstream      string `yaml:"upstream"`
	DebugUpstream string `yaml:"debug_upstream"`
	Moonraker     string `yaml:"moonraker"`
	Token         *Token `yaml:"token"`
}

var config *Config

var debug = flag.Bool("debug", false, "Debug on localhost")

func LoadConfig(file string) error {
	flag.Parse()

	result := &Config{}
	data, err := ioutil.ReadFile(file)

	if err != nil {
		return fmt.Errorf("Failed to load config %v\n", err)
	}

	err = yaml.Unmarshal([]byte(data), result)
	if err != nil {
		return fmt.Errorf("Failed to parse config %v\n", err)
	}

	config = result
	return nil
}

func (c Config) GetHostname() string {
	if *debug {
		return c.DebugHostname
	}
	return c.Hostname
}

func (c Config) GetUpstream() string {
	if *debug {
		return c.DebugUpstream
	}
	return c.Upstream
}

func StoreToken(file string, token *Token) error {
	cfg := GetConfig()
	cfg.Token = token
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("Failed to serialize config %v\n", err)
	}
	ioutil.WriteFile(file, data, 0)
	if err != nil {
		return fmt.Errorf("Failed to write config %v\n", err)
	}
	return nil
}

func GetConfig() *Config {
	return config
}
