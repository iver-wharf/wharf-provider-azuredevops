package main

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/gin-contrib/cors"
	"github.com/iver-wharf/wharf-core/pkg/cacertutil"
	"github.com/iver-wharf/wharf-core/pkg/ginutil"
	"github.com/iver-wharf/wharf-core/pkg/logger"
	"github.com/iver-wharf/wharf-core/pkg/logger/consolepretty"
	"github.com/iver-wharf/wharf-provider-azuredevops/docs"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

var log = logger.NewScoped("WHARF-PROVIDER-AZUREDEVOPS")

// @title Wharf provider API for Azure DevOps
// @description Wharf backend API for integrating Azure DevOps repositories
// @description with the Wharf main API.
// @license.name MIT
// @license.url https://github.com/iver-wharf/wharf-provider-azuredevops/blob/master/LICENSE
// @contact.name Iver Wharf Azure DevOps provider API support
// @contact.url https://github.com/iver-wharf/wharf-provider-azuredevops/issues
// @contact.email wharf@iver.se
// @basePath /import
func main() {
	logger.AddOutput(logger.LevelDebug, consolepretty.Default)

	var (
		config Config
		err    error
	)
	if err = loadEmbeddedVersionFile(); err != nil {
		log.Error().WithError(err).Message("Failed to read embedded version.yaml.")
		os.Exit(1)
	}
	if config, err = loadConfig(); err != nil {
		log.Error().WithError(err).Message("Failed to read config.")
		os.Exit(1)
	}

	docs.SwaggerInfo.Version = AppVersion.Version

	if config.CA.CertsFile != "" {
		client, err := cacertutil.NewHTTPClientWithCerts(config.CA.CertsFile)
		if err != nil {
			log.Error().WithError(err).Message("Failed to get net/http.Client with certs.")
			os.Exit(1)
		}
		http.DefaultClient = client
	}

	gin.DefaultWriter = ginutil.DefaultLoggerWriter
	gin.DefaultErrorWriter = ginutil.DefaultLoggerWriter

	r := gin.New()
	r.Use(
		ginutil.DefaultLoggerHandler,
		ginutil.RecoverProblem,
	)

	if config.HTTP.CORS.AllowAllOrigins {
		log.Info().Message("Allowing all origins in CORS.")
		r.Use(cors.Default())
	}

	r.GET("/", pingHandler)
	r.GET("/import/azuredevops/version", getVersionHandler)
	r.GET("/import/azuredevops/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	importModule{&config}.register(r)

	if err := r.Run(config.HTTP.BindAddress); err != nil {
		log.Error().
			WithError(err).
			WithString("address", config.HTTP.BindAddress).
			Message("Failed to start web server.")
		os.Exit(2)
	}
}

func pingHandler(c *gin.Context) {
	c.JSON(200, gin.H{"message": "pong"})
}
