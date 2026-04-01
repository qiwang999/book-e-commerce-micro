package config

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"sync"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/spf13/viper"
)

const (
	ConsulKVKey      = "bookhive/config"
	EnvConsulAddress = "CONSUL_ADDRESS"
	DefaultConsulAddr = "127.0.0.1:8500"
)

var (
	once      sync.Once
	mu        sync.RWMutex
	global    *Config
	watchStop chan struct{}
	onChange  []func(*Config)
)

type Config struct {
	App           AppConfig           `mapstructure:"app"`
	Consul        ConsulConfig        `mapstructure:"consul"`
	MySQL         MySQLConfig         `mapstructure:"mysql"`
	MongoDB       MongoDBConfig       `mapstructure:"mongodb"`
	Redis         RedisConfig         `mapstructure:"redis"`
	RabbitMQ      RabbitMQConfig      `mapstructure:"rabbitmq"`
	Elasticsearch ElasticsearchConfig `mapstructure:"elasticsearch"`
	Milvus        MilvusConfig        `mapstructure:"milvus"`
	MinIO         MinIOConfig         `mapstructure:"minio"`
	JWT           JWTConfig           `mapstructure:"jwt"`
	Email         EmailConfig         `mapstructure:"email"`
	OpenAI        OpenAIConfig        `mapstructure:"openai"`
	Services      ServicesConfig      `mapstructure:"services"`
	Sharding      ShardingConfig      `mapstructure:"sharding"`
}

type ShardingConfig struct {
	Order     ShardGroupConfig `mapstructure:"order"`
	Inventory ShardGroupConfig `mapstructure:"inventory"`
}

type ShardGroupConfig struct {
	Enabled bool          `mapstructure:"enabled"`
	Shards  []MySQLConfig `mapstructure:"shards"`
}

type MilvusConfig struct {
	Address string `mapstructure:"address"`
}

type MinIOConfig struct {
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Bucket    string `mapstructure:"bucket"`
	UseSSL    bool   `mapstructure:"use_ssl"`
}

type AppConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
}

type ConsulConfig struct {
	Address string `mapstructure:"address"`
}

type MySQLConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	Database     string `mapstructure:"database"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

func (c MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.User, c.Password, c.Host, c.Port, c.Database)
}

type MongoDBConfig struct {
	URI      string `mapstructure:"uri"`
	Database string `mapstructure:"database"`
}

type RedisConfig struct {
	Address  string `mapstructure:"address"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type RabbitMQConfig struct {
	URL string `mapstructure:"url"`
}

type ElasticsearchConfig struct {
	Addresses []string `mapstructure:"addresses"`
}

type JWTConfig struct {
	Secret      string `mapstructure:"secret"`
	ExpireHours int    `mapstructure:"expire_hours"`
}

type EmailConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
}

type OpenAIConfig struct {
	APIKey         string `mapstructure:"api_key"`
	Model          string `mapstructure:"model"`
	EmbeddingModel string `mapstructure:"embedding_model"`
	BaseURL        string `mapstructure:"base_url"`
}

type ServicesConfig struct {
	Gateway   ServicePortConfig `mapstructure:"gateway"`
	User      ServicePortConfig `mapstructure:"user"`
	Store     ServicePortConfig `mapstructure:"store"`
	Book      ServicePortConfig `mapstructure:"book"`
	Inventory ServicePortConfig `mapstructure:"inventory"`
	Cart      ServicePortConfig `mapstructure:"cart"`
	Order     ServicePortConfig `mapstructure:"order"`
	Payment   ServicePortConfig `mapstructure:"payment"`
	AI        ServicePortConfig `mapstructure:"ai"`
}

type ServicePortConfig struct {
	HTTPPort int    `mapstructure:"http_port"`
	GRPCPort int    `mapstructure:"grpc_port"`
	Host     string `mapstructure:"host"` // 可选，用于 Consul 注册的对外地址，空则用 127.0.0.1
}

// OnChange registers a callback that fires when config is reloaded from Consul KV.
func OnChange(fn func(*Config)) {
	mu.Lock()
	defer mu.Unlock()
	onChange = append(onChange, fn)
}

// Load reads configuration with the following priority:
//  1. Consul KV (key: bookhive/config) -- centralized config center
//  2. Local config.yaml file -- fallback when Consul is unavailable
//
// After loading, it starts a background goroutine to watch Consul KV
// for changes, enabling hot-reload without restarting services.
func Load(paths ...string) (*Config, error) {
	var err error
	once.Do(func() {
		consulAddr := getConsulAddr()

		cfg, e := loadFromConsul(consulAddr)
		if e == nil {
			global = cfg
			log.Printf("[config] loaded from Consul KV (%s, key=%s)", consulAddr, ConsulKVKey)
			watchStop = make(chan struct{})
			go watchConsul(consulAddr, watchStop)
			return
		}
		log.Printf("[config] Consul KV unavailable (%v), falling back to local file", e)

		cfg, e = loadFromFile(paths...)
		if e != nil {
			err = e
			return
		}
		global = cfg
		log.Println("[config] loaded from local config.yaml")
	})
	return global, err
}

// Get returns the current config. Panics if not loaded.
func Get() *Config {
	mu.RLock()
	c := global
	mu.RUnlock()
	if c == nil {
		cfg, err := Load()
		if err != nil {
			panic("config not loaded: " + err.Error())
		}
		return cfg
	}
	return c
}

// StopWatch stops the Consul KV watcher goroutine.
func StopWatch() {
	if watchStop != nil {
		close(watchStop)
	}
}

func getConsulAddr() string {
	if addr := os.Getenv(EnvConsulAddress); addr != "" {
		return addr
	}

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("../..")
	v.AddConfigPath("../../..")
	if err := v.ReadInConfig(); err == nil {
		if addr := v.GetString("consul.address"); addr != "" {
			return addr
		}
	}
	return DefaultConsulAddr
}

func loadFromConsul(addr string) (*Config, error) {
	client, err := consulapi.NewClient(&consulapi.Config{Address: addr})
	if err != nil {
		return nil, fmt.Errorf("create consul client: %w", err)
	}

	pair, _, err := client.KV().Get(ConsulKVKey, nil)
	if err != nil {
		return nil, fmt.Errorf("read consul kv: %w", err)
	}
	if pair == nil {
		return nil, fmt.Errorf("key %q not found in consul kv", ConsulKVKey)
	}

	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(bytes.NewReader(pair.Value)); err != nil {
		return nil, fmt.Errorf("parse consul config: %w", err)
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal consul config: %w", err)
	}
	return cfg, nil
}

func loadFromFile(paths ...string) (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	if len(paths) > 0 {
		for _, p := range paths {
			v.AddConfigPath(p)
		}
	}
	v.AddConfigPath(".")
	v.AddConfigPath("../..")
	v.AddConfigPath("../../..")

	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read local config: %w", err)
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal local config: %w", err)
	}
	return cfg, nil
}

// watchConsul uses Consul blocking queries to detect KV changes and hot-reload config.
func watchConsul(addr string, stop chan struct{}) {
	client, err := consulapi.NewClient(&consulapi.Config{Address: addr})
	if err != nil {
		log.Printf("[config] watch: failed to create consul client: %v", err)
		return
	}

	var lastIndex uint64
	for {
		select {
		case <-stop:
			log.Println("[config] watch stopped")
			return
		default:
		}

		pair, meta, err := client.KV().Get(ConsulKVKey, &consulapi.QueryOptions{
			WaitIndex: lastIndex,
		})
		if err != nil {
			log.Printf("[config] watch: consul query error: %v", err)
			continue
		}
		if pair == nil {
			continue
		}
		if meta.LastIndex == lastIndex {
			continue
		}
		lastIndex = meta.LastIndex

		v := viper.New()
		v.SetConfigType("yaml")
		if err := v.ReadConfig(bytes.NewReader(pair.Value)); err != nil {
			log.Printf("[config] watch: parse error: %v", err)
			continue
		}

		newCfg := &Config{}
		if err := v.Unmarshal(newCfg); err != nil {
			log.Printf("[config] watch: unmarshal error: %v", err)
			continue
		}

		mu.Lock()
		global = newCfg
		callbacks := make([]func(*Config), len(onChange))
		copy(callbacks, onChange)
		mu.Unlock()

		log.Println("[config] hot-reloaded from Consul KV")

		for _, fn := range callbacks {
			fn(newCfg)
		}
	}
}

