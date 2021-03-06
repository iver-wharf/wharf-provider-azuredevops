package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/iver-wharf/wharf-api-client-go/v2/pkg/wharfapi"
	"github.com/iver-wharf/wharf-core/pkg/ginutil"
	"github.com/iver-wharf/wharf-core/pkg/problem"
	_ "github.com/iver-wharf/wharf-provider-azuredevops/docs"
	"github.com/iver-wharf/wharf-provider-azuredevops/internal/azureapi"
	"github.com/iver-wharf/wharf-provider-azuredevops/internal/importer"
)

const (
	providerName = "azuredevops"
)

type importModule struct {
	config *Config
}

func (m importModule) register(r gin.IRouter) {
	r.POST("/import/azuredevops", m.runAzureDevOpsHandler)
	r.POST("/import/azuredevops/triggers/:projectid/pr/created", m.prCreatedTriggerHandler)
}

type importBody struct {
	// used in refresh only
	TokenID  uint   `json:"tokenId" example:"0"`
	Token    string `json:"token" example:"sample token"`
	UserName string `json:"user" example:"sample user name"`
	URL      string `json:"url" example:"https://gitlab.local"`
	// used in refresh only
	ProviderID uint `json:"providerId" example:"0"`
	// used in refresh only
	ProjectID   uint   `json:"projectId" example:"0"`
	ProjectName string `json:"project" example:"sample project name"`
	GroupName   string `json:"group" example:"default"`
}

// runAzureDevOpsHandler godoc
// @Summary Import projects from Azure DevOps or refresh existing one
// @Accept json
// @Produce json
// @Param import body importBody _ "import object"
// @Success 201 "Successfully imported"
// @Failure 400 {object} problem.Response "Bad request"
// @Failure 401 {object} problem.Response "Unauthorized or missing jwt token"
// @Failure 502 {object} problem.Response "Bad gateway"
// @Router /azuredevops [post]
func (m importModule) runAzureDevOpsHandler(c *gin.Context) {
	client := wharfapi.Client{
		APIURL:     m.config.API.URL,
		AuthHeader: c.GetHeader("Authorization"),
	}

	i := importBody{}
	err := c.ShouldBindJSON(&i)
	if err != nil {
		ginutil.WriteInvalidBindError(c, err,
			"One or more parameters failed to parse when reading the request body for import details.")
		return
	}

	if i.GroupName == "" {
		log.Error().Message("Unable to get due to empty group.")
		err := errors.New("missing required property: group")
		ginutil.WriteInvalidParamError(c, err, "group",
			"Unable to import due to empty group.")
		return
	}

	tokenData := importer.TokenData{
		ReqToken: importer.ReqToken{
			Token:    i.Token,
			UserName: i.UserName,
		},
		ID: i.TokenID,
	}
	providerData := importer.ProviderData{
		ReqProvider: importer.ReqProvider{
			Name:    providerName,
			URL:     i.URL,
			TokenID: i.TokenID,
		},
		ID: i.ProviderID,
	}

	importer := importer.NewAzureImporter(c, &client)
	ok := importer.InitWritesProblem(tokenData, providerData, c, client)
	if !ok {
		return
	}

	azureOrg, azureProj, azureRepo := parseRepoRefParams(i.GroupName, i.ProjectName)
	switch {
	case azureProj == "":
		log.Debug().
			WithString("org", azureOrg).
			Message("Importing all repos from org")
		ok = importer.ImportOrganizationWritesProblem(azureOrg)
	case azureRepo == "":
		log.Debug().
			WithString("org", azureOrg).
			WithString("project", azureProj).
			Message("Importing all repos from project")
		ok = importer.ImportProjectWritesProblem(azureOrg, azureProj)
	default:
		log.Debug().
			WithString("org", azureOrg).
			WithString("project", azureProj).
			WithString("repo", azureRepo).
			Message("Importing specific repo from project")
		ok = importer.ImportRepositoryWritesProblem(azureOrg, azureProj, azureRepo)
	}

	if !ok {
		return
	}

	c.Status(http.StatusCreated)
}

func parseRepoRefParams(wharfGroupName, wharfProjectName string) (azureOrgName, azureProjectName, azureRepoName string) {
	azureOrgName, azureProjectName = splitStringOnceRune(wharfGroupName, '/')
	if azureProjectName == "" {
		azureProjectName = wharfProjectName
		azureRepoName = ""
	} else {
		azureRepoName = wharfProjectName
	}
	return
}

// prCreatedTriggerHandler godoc
// @Summary Triggers prcreated action on wharf-client
// @Accept json
// @Produce json
// @Param projectid path int true "wharf project ID"
// @Param azureDevOpsPR body azureapi.PullRequestEvent _ "AzureDevOps PR"
// @Param environment query string true "wharf build environment"
// @Success 200 {object} response.BuildReferenceWrapper "OK"
// @Failure 400 {object} problem.Response "Bad request"
// @Failure 401 {object} problem.Response "Unauthorized or missing jwt token"
// @Failure 502 {object} problem.Response "Bad gateway"
// @Router /azuredevops/triggers/{projectid}/pr/created [post]
func (m importModule) prCreatedTriggerHandler(c *gin.Context) {
	const eventTypePullRequest string = "git.pullrequest.created"

	t := azureapi.PullRequestEvent{}
	if err := c.ShouldBindJSON(&t); err != nil {
		ginutil.WriteInvalidBindError(c, err,
			"One or more parameters failed to parse when reading the request body for pull request.")
		return
	}

	if t.EventType != eventTypePullRequest {
		err := fmt.Errorf("expected event type %q for trigger, got: %q", eventTypePullRequest, t.EventType)
		ginutil.WriteProblemError(c, err, problem.Response{
			Type:   "/prob/provider/azuredevops/unsupported-event-type",
			Title:  "Invalid event type.",
			Status: http.StatusBadRequest,
			Detail: fmt.Sprintf("Received event type %q, while only %q is supported.",
				t.EventType, eventTypePullRequest),
		})
		return
	}

	projectID, ok := ginutil.ParseParamUint(c, "projectid")
	if !ok {
		return
	}

	environment, ok := ginutil.RequireQueryString(c, "environment")
	if !ok {
		return
	}

	client := wharfapi.Client{
		APIURL:     m.config.API.URL,
		AuthHeader: c.GetHeader("Authorization"),
	}

	params := wharfapi.ProjectStartBuild{
		Stage:       "prcreated",
		Branch:      strings.TrimPrefix(t.Resource.SourceRefName, "refs/heads/"),
		Environment: environment,
	}
	resp, err := client.StartProjectBuild(projectID, params, nil)

	if authErr, ok := err.(*wharfapi.AuthError); ok {
		ginutil.WriteUnauthorizedError(c, authErr,
			"Failed to authenticate to the Wharf API. The Authorization header was "+
				"missing or is invalid.")
		return
	}

	if err != nil {
		log.Error().WithError(err).Message("Failed to send trigger to wharf-api.")
		err = fmt.Errorf("unable to send trigger to wharf-api: %w", err)
		ginutil.WriteTriggerError(c, err, "Unable to send trigger to Wharf API.")
		return
	}

	c.JSON(http.StatusOK, resp)
}
