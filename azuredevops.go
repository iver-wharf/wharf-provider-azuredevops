package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/iver-wharf/wharf-api-client-go/pkg/wharfapi"
	"github.com/iver-wharf/wharf-core/pkg/ginutil"
	"github.com/iver-wharf/wharf-core/pkg/problem"
	_ "github.com/iver-wharf/wharf-provider-azuredevops/docs"
	"github.com/iver-wharf/wharf-provider-azuredevops/internal/azureapi"
	"github.com/iver-wharf/wharf-provider-azuredevops/internal/importer"
)

type importBody struct {
	// used in refresh only
	TokenID   uint   `json:"tokenId" example:"0"`
	Token     string `json:"token" example:"sample token"`
	UserName  string `json:"user" example:"sample user name"`
	URL       string `json:"url" example:"https://gitlab.local"`
	UploadURL string `json:"uploadUrl" example:""`
	// used in refresh only
	ProviderID uint `json:"providerId" example:"0"`
	// azuredevops, gitlab or github
	ProviderName string `json:"provider" example:"gitlab"`
	// used in refresh only
	ProjectID   uint   `json:"projectId" example:"0"`
	ProjectName string `json:"project" example:"sample project name"`
	GroupName   string `json:"group" example:"default"`
}

// runAzureDevOpsHandler godoc
// @Summary Import projects from Azure DevOps or refresh existing one
// @Accept json
// @Produce json
// @Param import body importData _ "import object"
// @Success 201 "Successfully imported"
// @Failure 400 {object} problem.Response "Bad request"
// @Failure 401 {object} problem.Response "Unauthorized or missing jwt token"
// @Failure 502 {object} problem.Response "Bad gateway"
// @Router /azuredevops [post]
func runAzureDevOpsHandler(c *gin.Context) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	client := wharfapi.Client{
		ApiUrl:     os.Getenv("WHARF_API_URL"),
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
		fmt.Println("Unable to get due to empty group.")
		err := errors.New("missing required property: group")
		ginutil.WriteInvalidParamError(c, err, "group",
			"Unable to import due to empty group.")
		return
	}

	fmt.Println("from json: ", i)

	importer := importer.NewAzureImporter(c, &client)
	token := wharfapi.Token{
		TokenID:    i.TokenID,
		Token:      i.Token,
		UserName:   i.UserName,
		ProviderID: i.ProviderID}
	provider := wharfapi.Provider{
		ProviderID: i.ProviderID,
		Name:       i.ProviderName,
		URL:        i.URL,
		UploadURL:  i.UploadURL,
		TokenID:    i.TokenID}

	ok := importer.InitWritesProblem(token, provider, c, client)
	if !ok {
		return
	}

	if i.ProjectName != "" {
		ok = importer.ImportProjectInGroupWritesProblem(i.GroupName, i.ProjectName)
	} else {
		ok = importer.ImportAllProjectsInGroupWritesProblem(i.GroupName)
	}

	if !ok {
		return
	}

	c.Status(http.StatusCreated)
}

// prCreatedTriggerHandler godoc
// @Summary Triggers prcreated action on wharf-client
// @Accept json
// @Produce json
// @Param projectid path int true "wharf project ID"
// @Param azureDevOpsPR body azureapi.PullRequestEvent _ "AzureDevOps PR"
// @Param environment query string true "wharf build environment"
// @Success 200 {object} wharfapi.ProjectRunResponse "OK"
// @Failure 400 {object} problem.Response "Bad request"
// @Failure 401 {object} problem.Response "Unauthorized or missing jwt token"
// @Failure 502 {object} problem.Response "Bad gateway"
// @Router /azuredevops/triggers/{projectid}/pr/created [post]
func prCreatedTriggerHandler(c *gin.Context) {
	const eventTypePullRequest string = "git.pullrequest.created"

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

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
		ApiUrl:     os.Getenv("WHARF_API_URL"),
		AuthHeader: c.GetHeader("Authorization"),
	}

	var resp wharfapi.ProjectRunResponse
	resp, err := client.PostProjectRun(
		wharfapi.ProjectRun{
			ProjectID:   projectID,
			Stage:       "prcreated",
			Branch:      strings.TrimPrefix(t.Resource.SourceRefName, "refs/heads/"),
			Environment: environment,
		},
	)

	if authErr, ok := err.(*wharfapi.AuthError); ok {
		ginutil.WriteUnauthorizedError(c, authErr,
			"Failed to authenticate to the Wharf API. The Authorization header was "+
			"missing or is invalid.")
			return
	}

	if err != nil {
		fmt.Println("Unable to send trigger to wharf-client: ", err)
		err = fmt.Errorf("unable to send trigger to wharf-client: %v", err)
		ginutil.WriteTriggerError(c, err, "Unable to send trigger to Wharf client.")
		return
	}

	c.JSON(http.StatusOK, resp)
}
