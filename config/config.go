package config

import (
    "os"
    "fmt"
    "errors"
    "encoding/json"
)

type Config struct {                                           
    TestMode bool `json:"testMode"`
    OpenAIAPIKey string `json:"openaiApiKey"`
    SSHPort int `json:"sshPort"`
    HTTPPort int `json:"httpPort"`
    NumberOfSentencesPerTypingTest int `json:"numberOfSentencesPerTypingTest"`                        
    TypingTestDesiredWidth int `json:"typingTestDesiredWidth"`
    RaceStartTimeoutInSeconds int `json:"raceStartTimeoutInSeconds"`
    MaxPlayersPerRace int8 `json:"maxPlayersPerRace"`
    HostKey string `json:"hostKey"`
    Database Database `json:"database"`
}                                                              

type Database struct {
    DataDirectory string `json:"dataDirectory"`
    MigrationDirectory string `json:"migrationDirectory"`
}


func New(configPath string) (Config, error) {
    bytes, err := os.ReadFile(configPath)
    if err != nil {
        return Config{}, fmt.Errorf("error, when reading config file. Error: %v", err)
    }
    c := Config{}
    err = json.Unmarshal(bytes, &c)
    if err != nil {
        return Config{}, fmt.Errorf("error, when unmarshaling config file. Error: %v", err)
    }
    if !c.isValid() {
        return Config{}, errors.New("error, invalid config, ensure all values are provided")
    }
    return c, nil
}

func (c *Config) isValid() bool {
    return c.OpenAIAPIKey != "" &&
        c.SSHPort != 0 &&
        c.HTTPPort != 0 &&
        c.NumberOfSentencesPerTypingTest != 0 &&
        c.TypingTestDesiredWidth > 5 &&
        c.HostKey != "" &&
        c.RaceStartTimeoutInSeconds != 0 && 
        c.MaxPlayersPerRace != 0 &&
        c.Database.DataDirectory != "" &&
        c.Database.MigrationDirectory != ""
}
