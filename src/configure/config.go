package configure

import (
	"bytes"
	"os"
	"reflect"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var Cfg = ServerCfg{}

type ServerCfg struct {
	Level      string `mapstructure:"level" json:"level"`
	ConfigFile string `mapstructure:"config_file" json:"config_file"`

	RedisURI string `mapstructure:"redis_uri" json:"redis_uri"`
	MongoURI string `mapstructure:"mongo_uri" json:"mongo_uri"`
	MongoDB  string `mapstructure:"mongo_db" json:"mongo_db"`

	ConnURI  string `mapstructure:"conn_uri" json:"conn_uri"`
	ConnType string `mapstructure:"conn_type" json:"conn_type"`

	WebsiteURL   string `mapstructure:"website_url" json:"website_url"`
	CookieDomain string `mapstructure:"cookie_domain" json:"cookie_domain"`
	CookieSecure string `mapstructure:"cookie_secure" json:"cookie_secure"`

	TwitchClientID     string `mapstructure:"twitch_client_id" json:"twitch_client_id"`
	TwitchRedirectURI  string `mapstructure:"twitch_redirect_uri" json:"twitch_redirect_uri"`
	TwitchClientSecret string `mapstructure:"twitch_client_secret" json:"twitch_client_secret"`

	TempFileStore string `mapstructure:"temp_file_store" json:"temp_file_store"`

	JWTSecret string `mapstructure:"jwt_secret" json:"jwt_secret"`

	AwsAKID      string `mapstructure:"aws_akid" json:"aws_akid"`
	AwsToken     string `mapstructure:"aws_session_token" json:"aws_session_token"`
	AwsSecretKey string `mapstructure:"aws_secret_key" json:"aws_secret_key"`
	AwsCDNBucket string `mapstructure:"aws_cdn_bucket" json:"aws_cdn_bucket"`
	AwsRegion    string `mapstructure:"aws_region" json:"aws_region"`
	AwsEndpoint  string `mapstructure:"aws_endpoint" json:"aws_endpoint"`

	NodeID string `mapstructure:"node_id" json:"node_id"`

	DisableRedisCache bool `mapstructure:"disable_redis_cache" json:"disable_redis_cache"`

	GqlSniffer string `mapstructure:"gql_sniffer" json:"gql_sniffer"`

	ExitCode int `mapstructure:"exit_code" json:"exit_code"`
}

// default config
var defaultConf = ServerCfg{
	ConfigFile: "config.yaml",
}

var Config = viper.New()

// Capture environment variables
var NodeName string = os.Getenv("NODE_NAME")
var PodName string = os.Getenv("POD_NAME")
var PodIP string = os.Getenv("POD_IP")

func initLog() {
	if l, err := logrus.ParseLevel(Config.GetString("level")); err == nil {
		logrus.SetLevel(l)
		logrus.SetReportCaller(true)
	}
}

func checkErr(err error) {
	if err != nil {
		logrus.WithError(err).Fatal("config")
	}
}

func init() {
	if NodeName == "" {
		NodeName = "STANDALONE"
	}
	if PodName == "" {
		PodName = "STANDALONE"
	}

	logrus.SetFormatter(&logrus.JSONFormatter{})
	// Default config
	b, _ := json.Marshal(defaultConf)
	defaultConfig := bytes.NewReader(b)
	viper.SetConfigType("json")
	checkErr(viper.ReadConfig(defaultConfig))
	checkErr(Config.MergeConfigMap(viper.AllSettings()))

	// Flags
	pflag.String("config_file", "config.yaml", "configure filename")
	pflag.String("level", "info", "Log level")
	pflag.String("redis_uri", "", "Address for the redis server.")
	pflag.String("mongo_uri", "", "Address for the mongo server.")
	pflag.String("mongo_db", "", "Database to use.")

	pflag.String("conn_uri", "", "Connection url:port or path")
	pflag.String("conn_type", "", "Connection type, udp/tcp/unix")

	pflag.String("website_url", "", "Url for the website")
	pflag.String("cookie_domain", "", "Domain for the cookies to be set.")
	pflag.Bool("cookie_secure", true, "Set a secure cookie.")

	pflag.String("twitch_client_id", "", "Twitch client id")
	pflag.String("twitch_redirect_uri", "", "Twitch redirect uri")
	pflag.String("twitch_client_secret", "", "Twitch client secret")

	pflag.String("temp_file_store", "", "The temp folder for saving images.")

	pflag.String("jwt_secret", "", "The JWT secret for auth.")

	pflag.String("aws_session_token", "", "AWS Session Token")
	pflag.String("aws_akid", "", "AWS AKID")
	pflag.String("aws_secret_key", "", "AWS SecretKey")
	pflag.String("aws_cdn_bucket", "", "AWS s3 bucket name for our cdn")
	pflag.String("aws_region", "", "AWS region")
	pflag.String("aws_endpoint", "", "AWS Endpoint")

	pflag.String("node_id", "", "Used in the response header of a requset X-Node-ID")

	pflag.Bool("disable_redis_cache", false, "Disable the redis cache for mongodb")

	pflag.String("gql_sniffer", "", "Url for the GQL Sniffer")

	pflag.String("version", "1.0", "Version of the system.")
	pflag.Int("exit_code", 0, "Status code for successful and graceful shutdown, [0-125].")
	pflag.Parse()
	checkErr(Config.BindPFlags(pflag.CommandLine))

	// File
	Config.SetConfigFile(Config.GetString("config_file"))
	Config.AddConfigPath(".")
	err := Config.ReadInConfig()
	if err != nil {
		logrus.Warning(err)
		logrus.Info("Using default config")
	} else {
		checkErr(Config.MergeInConfig())
	}

	BindEnvs(Config, ServerCfg{})

	// Environment
	Config.AutomaticEnv()
	Config.SetEnvPrefix("SERVERGO")
	Config.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	Config.AllowEmptyEnv(true)
	// Log
	initLog()

	// Print final config
	checkErr(Config.Unmarshal(&Cfg))
}

func BindEnvs(config *viper.Viper, iface interface{}, parts ...string) {
	ifv := reflect.ValueOf(iface)
	ift := reflect.TypeOf(iface)
	for i := 0; i < ift.NumField(); i++ {
		v := ifv.Field(i)
		t := ift.Field(i)
		tv, ok := t.Tag.Lookup("mapstructure")
		if !ok {
			continue
		}
		switch v.Kind() {
		case reflect.Struct:
			BindEnvs(config, v.Interface(), append(parts, tv)...)
		default:
			_ = config.BindEnv(strings.Join(append(parts, tv), "."))
		}
	}
}
