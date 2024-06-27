package config

import (
	"log"
	"os"

	"github.com/RackSec/srslog"
	"github.com/go-playground/validator"
	"github.com/mattn/go-colorable"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Config struct {
	AppName            string         `mapstructure:"APP_NAME" validate:"required"`
	GoServicePort      string         `mapstructure:"GO_SERVICE_PORT" validate:"required"`
	NodeUUID           string         `json:"-"`
	EtcdUrl            string         `mapstructure:"ETCD_URL" validate:"required" json:"-"`
	SysLog             string         `mapstructure:"SYSLOG" validate:"required" json:"-"`
	CorsWhitelist      string         `mapstructure:"CORS_WHITELIST" validate:"required" json:"-"`
	PerRequestLimit    string         `mapstructure:"PER_REQUEST_LIMIT" validate:"required"`
	CacheExpiry        string         `mapstructure:"CACHE_EXPIRY" validate:"required"`
	AllowAllOrigins    bool           `mapstructure:"ALLOW_ALL_ORIGINS" json:"-"`
	AllowedOrigins     []string       `mapstructure:"ALLOWED_ORIGINS" validate:"required" json:"-"`
	AllowedRestMethods []string       `mapstructure:"ALLOWED_REST_METHODS" validate:"required"`
	AllowedRestHeaders []string       `mapstructure:"ALLOWED_REST_HEADERS" validate:"required"`
	AllowedSpecificIps []string       `mapstructure:"ALLOWED_SPECIFIC_IPS" validate:"required"`
	AllowedIpRanges    []string       `mapstructure:"ALLOWED_IP_RANGES" validate:"required"`
	SubServices        []string       `mapstructure:"SUB_SERVICES" validate:"required"`
	EtcdNodes          int            `json:"-"`
	IsLeader           chan bool      `json:"-"`
	Logger             *zap.Logger    `json:"-"`
	LoggerSys          *srslog.Writer `json:"-"`
	ServiceName        string         `json:"ServiceName"`
}

var config = &Config{}

func init() {
	viper.SetConfigName("config")
	viper.AddConfigPath("./internal/config/")
	viper.SetConfigType("json")

	log.Println("Reading config...")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Fatal error config: %v", err)
	}

	log.Println("Unmarshalling config...")
	err = viper.Unmarshal(&config)
	if err != nil {
		log.Fatalf("Unable to decode into struct, %v", err)
	}

	if config.SysLog == "true" {

		config.LoggerSys, err = srslog.Dial("", "", srslog.LOG_INFO, "CEF0")
		if err != nil {
			log.Println("Error setting up syslog:", err)
			os.Exit(1)
		}

	}

	logs := zap.NewDevelopmentEncoderConfig() //zap.NewProduction()
	logs.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.Logger = zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(logs),
		zapcore.AddSync(colorable.NewColorableStdout()),
		zapcore.DebugLevel,
	))
	defer config.Logger.Sync()

	validate := validator.New()
	err = validate.Struct(config)
	if err != nil {
		log.Fatalf("Config validation failed, %v", err)
	}
}

func GetConfig() (*Config, error) {
	return config, nil
}
