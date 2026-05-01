package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Credentials struct {
	Anthropic string `json:"anthropic_api_key,omitempty"`
	OpenAI    string `json:"openai_api_key,omitempty"`
	Groq      string `json:"groq_api_key,omitempty"`
}

func CredentialsPath() string {
	return filepath.Join(GlobalDir(), "credentials.json")
}

func LoadCredentials() (*Credentials, error) {
	c := &Credentials{}
	data, err := os.ReadFile(CredentialsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return c, err
	}
	if err := json.Unmarshal(data, c); err != nil {
		return c, err
	}
	return c, nil
}

func SaveCredentials(c *Credentials) error {
	if err := os.MkdirAll(GlobalDir(), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(CredentialsPath(), data, 0600)
}

func (c *Credentials) Set(provider, key string) {
	switch provider {
	case "anthropic":
		c.Anthropic = key
	case "openai":
		c.OpenAI = key
	case "groq":
		c.Groq = key
	}
}
