package config

import (
	"os"
)

// Config is the top-level configuration struct for mortar
type Config struct {
	//GRPC     GRPC
	HTTP     HTTP
	Database Database
}

// Database store database configuration information (currently just for postgres)
type Database struct {
	Host     string
	Database string
	User     string
	Password string
	Port     string
}

// type GRPC struct {
// 	ListenAddress string
// 	Port          string
// }

// HTTP stores config information for the HTTP interface
type HTTP struct {
	ListenAddress string
	Port          string
}

// NewFromEnv creates a new config from environment variables
func NewFromEnv() *Config {
	return &Config{
		//GRPC: GRPC{
		//	ListenAddress: os.Getenv("MORTAR_GRPC_ADDRESS"),
		//	Port:          os.Getenv("MORTAR_GRPC_PORT"),
		//},
		HTTP: HTTP{
			ListenAddress: os.Getenv("MORTAR_HTTP_ADDRESS"),
			Port:          os.Getenv("MORTAR_HTTP_PORT"),
		},
		Database: Database{
			Host:     os.Getenv("MORTAR_DB_HOST"),
			Database: os.Getenv("MORTAR_DB_DATABASE"),
			User:     os.Getenv("MORTAR_DB_USER"),
			Password: os.Getenv("MORTAR_DB_PASSWORD"),
			Port:     os.Getenv("MORTAR_DB_PORT"),
		},
	}
}
