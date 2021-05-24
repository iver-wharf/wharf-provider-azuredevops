package main

import (
	"fmt"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/gin-contrib/cors"
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

// @title Swagger import API
// @version 1.0
// @description Wharf import server.

// @Host
// @BasePath /import
func main() {
	r := gin.Default()

	allowCORS, ok := os.LookupEnv("ALLOW_CORS")
	if ok && allowCORS == "YES" {
		fmt.Printf("Allowing CORS\n")
		r.Use(cors.Default())
	}

	r.GET("/", pingHandler)
	r.POST("/import/azuredevops/triggers/:projectid/pr/created", prCreatedTriggerHandler)
	r.POST("/import/azuredevops", runAzureDevOpsHandler)
	r.GET("/import/azuredevops/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.Run()
}

func pingHandler(c *gin.Context) {
	c.JSON(200, gin.H{"message": "pong"})
}
