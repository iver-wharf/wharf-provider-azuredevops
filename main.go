package main

import (
	"fmt"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/gin-contrib/cors"
	"github.com/iver-wharf/wharf-provider-azuredevops/docs"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type importBody struct {
	// used in refresh only
	TokenID   uint   `json:"tokenId" example:"0"`
	Token     string `json:"token" example:"sample token"`
	User      string `json:"user" example:"sample user name"`
	URL       string `json:"url" example:"https://gitlab.local"`
	UploadURL string `json:"uploadUrl" example:""`
	// used in refresh only
	ProviderID uint `json:"providerId" example:"0"`
	// azuredevops, gitlab or github
	Provider string `json:"provider" example:"gitlab"`
	// used in refresh only
	ProjectID uint   `json:"projectId" example:"0"`
	Project   string `json:"project" example:"sample project name"`
	Group     string `json:"group" example:"default"`
}

const buildDefinitionFileName = ".wharf-ci.yml"

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
	if err := loadEmbeddedVersionFile(); err != nil {
		fmt.Println("Failed to read embedded version.yaml file:", err)
		os.Exit(1)
	}

	docs.SwaggerInfo.Version = AppVersion.Version

	r := gin.Default()

	allowCORS, ok := os.LookupEnv("ALLOW_CORS")
	if ok && allowCORS == "YES" {
		fmt.Printf("Allowing CORS\n")
		r.Use(cors.Default())
	}

	r.GET("/", pingHandler)
	r.POST("/import/azuredevops/triggers/:projectid/pr/created", prCreatedTriggerHandler)
	r.POST("/import/azuredevops", runAzureDevOpsHandler)
	r.GET("/import/azuredevops/version", getVersionHandler)
	r.GET("/import/azuredevops/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.Run(getBindAddress())
}

func getBindAddress() string {
	bindAddress, isBindAddressDefined := os.LookupEnv("BIND_ADDRESS")
	if !isBindAddressDefined || bindAddress == "" {
		return "0.0.0.0:8080"
	}
	return bindAddress
}

func pingHandler(c *gin.Context) {
	c.JSON(200, gin.H{"message": "pong"})
}
