package main

import (
	"os"

	"github.com/iver-wharf/wharf-core/pkg/config"
	"github.com/iver-wharf/wharf-core/pkg/env"
)

// Config holds all configurable settings for wharf-provider-azuredevops.
//
// The config is read in the following order:
//
// 1. File: /etc/iver-wharf/wharf-provider-azuredevops/config.yml
//
// 2. File: ./wharf-provider-azuredevops-config.yml
//
// 3. File from environment variable: WHARF_CONFIG
//
// 4. Environment variables, prefixed with WHARF_
//
// Each inner struct is represented as a deeper field in the different
// configurations. For YAML they represent deeper nested maps. For environment
// variables they are joined together by underscores.
//
// All environment variables must be uppercased, while YAML files are
// case-insensitive. Keeping camelCasing in YAML config files is recommended
// for consistency.
type Config struct {
	API  WharfAPIConfig
	HTTP HTTPConfig
	CA   CertConfig
}

// WharfAPIConfig holds settings for the connection to the Wharf API.
type WharfAPIConfig struct {
	// URL is the base URL targetted towards the Wharf API. In a standard
	// installation of Wharf, this would include the trailing "/api" in the URL
	// path.
	//
	// Added in v1.3.0.
	URL string
}

// HTTPConfig holds settings for the HTTP server.
type HTTPConfig struct {
	CORS CORSConfig

	// BindAddress is the IP-address and port, separated by a colon, to bind
	// the HTTP server to. An IP-address of 0.0.0.0 will bind to all
	// IP-addresses.
	//
	// For backward compatibility, that may be removed in the next major release
	// (v3.0.0), the environment variable BIND_ADDRESS, which was added in
	// v2.0.0, will also set this value.
	//
	// Added in v1.3.0
	BindAddress string
}

// CORSConfig holds settings for the HTTP server's CORS settings.
type CORSConfig struct {
	// AllowAllOrigins enables CORS and allows all hostnames and URLs in the
	// HTTP request origins when set to true. Practically speaking, this
	// results in the HTTP header "Access-Control-Allow-Origin" set to "*".
	//
	// For backward compatibility, that may be removed in the next major release
	// (v3.0.0), the environment variable ALLOW_CORS, which was added in v0.6.0,
	// when set to "YES" will then set this value to true.
	//
	// Added in v1.3.0
	AllowAllOrigins bool
}

// CertConfig holds settings for certificates verification used when talking
// to remote services over HTTPS.
type CertConfig struct {
	// CertsFile points to a file of one or more PEM-formatted certificates to
	// use in addition to the certificates from the system
	// (such as from /etc/ssl/certs/).
	//
	// Added in v1.3.0
	CertsFile string

	/// InsecureSkipVerify will disable all TLS certificate verification for
	/// outgoing traffic from the wharf-provider-azuredevops application.
	///
	/// This is a major security hole and should be avoided whenever possible.
	///
	/// Added in v2.0.0.
	InsecureSkipVerify bool
}

// DefaultConfig is the hard-coded default values for wharf-provider-azuredevops's
// configs.
var DefaultConfig = Config{
	HTTP: HTTPConfig{
		BindAddress: "0.0.0.0:8080",
	},
}

func loadConfig() (Config, error) {
	cfgBuilder := config.NewBuilder(DefaultConfig)

	cfgBuilder.AddConfigYAMLFile("/etc/iver-wharf/wharf-provider-azuredevops/config.yml")
	cfgBuilder.AddConfigYAMLFile("wharf-provider-azuredevops-config.yml")
	if cfgFile, ok := os.LookupEnv("WHARF_CONFIG"); ok {
		cfgBuilder.AddConfigYAMLFile(cfgFile)
	}
	cfgBuilder.AddEnvironmentVariables("WHARF")

	var (
		cfg Config
		err = cfgBuilder.Unmarshal(&cfg)
	)
	if err == nil {
		err = cfg.addBackwardCompatibleConfigs()
	}
	return cfg, err
}

func (cfg *Config) addBackwardCompatibleConfigs() error {
	if value, ok := os.LookupEnv("ALLOW_CORS"); ok && value == "YES" {
		cfg.HTTP.CORS.AllowAllOrigins = true
	}
	return env.Bind(&cfg.HTTP.BindAddress, "BIND_ADDRESS")
}
