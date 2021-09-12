package main

type IConfig interface {
	PersisterConfig() IPersisterConfig
	CacherConfig1() ICacherConfig
	CacherConfig2() ICacherConfig
	CacherConfig3() ICacherConfig
	CacherConfig4() ICacherConfig
	CacherConfig5() ICacherConfig
}

type Config struct{}

func NewConfig() IConfig {
	return &Config{}
}

func (cfg *Config) PersisterConfig() IPersisterConfig {
	return NewPersisterConfig()
}

func (cfg *Config) CacherConfig1() ICacherConfig {
	return NewCacherConfig("127.0.0.1:6379")
}

func (cfg *Config) CacherConfig2() ICacherConfig {
	return NewCacherConfig("127.0.0.1:6380")
}

func (cfg *Config) CacherConfig3() ICacherConfig {
	return NewCacherConfig("127.0.0.1:6381")
}

func (cfg *Config) CacherConfig4() ICacherConfig {
	return NewCacherConfig("127.0.0.1:6382")
}

func (cfg *Config) CacherConfig5() ICacherConfig {
	return NewCacherConfig("127.0.0.1:6383")
}

type CacherConfig struct {
	endpoint string
}

func NewCacherConfig(endpoint string) *CacherConfig {
	return &CacherConfig{
		endpoint: endpoint,
	}
}

func (cfg *CacherConfig) Endpoint() string {
	return cfg.endpoint
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
