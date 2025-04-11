package config

import (
    "log"
    "os"

    "gopkg.in/yaml.v3"
)

type Config struct {
    Telegram struct {
        BotToken string `yaml:"bot_token"`
        GroupID  int64  `yaml:"group_id"`
    } `yaml:"telegram"`

    Spotify struct {
        ClientID     string `yaml:"client_id"`
        ClientSecret string `yaml:"client_secret"`
        RedirectURL  string `yaml:"redirect_url"`
        PlaylistID   string `yaml:"playlist_id"`
    } `yaml:"spotify"`
}

func Load(path string) *Config {
    f, err := os.Open(path)
    if err != nil {
        log.Fatalf("failed to open config: %v", err)
    }
    defer f.Close()

    var cfg Config
    decoder := yaml.NewDecoder(f)
    if err := decoder.Decode(&cfg); err != nil {
        log.Fatalf("failed to decode config: %v", err)
    }
    return &cfg
}