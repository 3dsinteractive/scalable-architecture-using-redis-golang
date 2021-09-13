package main

type IConfig interface {
	PersisterConfig() IPersisterConfig
	CacherConfig() ICacherConfig
}

type Config struct{}

func NewConfig() IConfig {
	return &Config{}
}

func (cfg *Config) PersisterConfig() IPersisterConfig {
	return NewPersisterConfig()
}

func (cfg *Config) CacherConfig() ICacherConfig {
	return NewCacherConfig()
}

type CacherConfig struct{}

func NewCacherConfig() *CacherConfig {
	return &CacherConfig{}
}

func (cfg *CacherConfig) Endpoint() string {
	return "127.0.0.1:6379"
}

func (cfg *CacherConfig) Password() string {
	return ""
}

func (cfg *CacherConfig) DB() int {
	return 0
}

func (cfg *CacherConfig) ConnectionSettings() ICacherConnectionSettings {
	return NewDefaultCacherConnectionSettings()
}

type PersisterConfig struct{}

func NewPersisterConfig() *PersisterConfig {
	return &PersisterConfig{}
}

func (cfg *PersisterConfig) Endpoint() string {
	return "127.0.0.1"
}

func (cfg *PersisterConfig) Port() string {
	return "3306"
}

func (cfg *PersisterConfig) DB() string {
	return "my_database"
}

func (cfg *PersisterConfig) Username() string {
	return "my_user"
}

func (cfg *PersisterConfig) Password() string {
	return "my_password"
}

func (cfg *PersisterConfig) Charset() string {
	return "utf8mb4"
}
