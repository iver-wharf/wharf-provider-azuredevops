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
	"github.com/iver-wharf/wharf-provider-azuredevops/internal/importer"
)

type azureDevOpsPR struct {
	EventType string `json:"eventType" example:"git.pullrequest.created"`
	Resource  struct {
		PullRequestID uint   `json:"pullRequestId" example:"1"`
		SourceRefName string `json:"sourceRefName" example:"refs/heads/master"`
	}
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
func runAzureDevOpsHandler(c *gin.Context) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	client := wharfapi.Client{
		ApiUrl:     os.Getenv("WHARF_API_URL"),
		AuthHeader: c.GetHeader("Authorization"),
	}

	i := importer.AzureDevOpsImporter{}
	err := c.ShouldBindJSON(&i)
	if err != nil {
		ginutil.WriteInvalidBindError(c, err,
			"One or more parameters failed to parse when reading the request body for import details.")
		return
	}

	fmt.Println("from json: ", i)

	if i.GroupName == "" {
		fmt.Println("Unable to get due to empty group.")
		err := errors.New("missing required property: group")
		ginutil.WriteInvalidParamError(c, err, "group",
			"Unable to import due to empty group.")
		return
	}

	var ok bool
	i.Token, ok = i.GetOrPostTokenWritesProblem(c, client)
	if !ok {
		fmt.Println("Unable to get or create token.")
		return
	}
	fmt.Println("Token from db: ", i.Token)

	i.Provider, ok = i.GetOrPostProviderWritesProblem(c, client)
	if !ok {
		return
	}
	fmt.Println("Provider from db: ", i.Provider)

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	var projects importer.AzureDevOpsProjectResponse
	if i.ProjectName != "" {
		projects, ok = i.GetProjectWritesProblem(c)
	} else {
		projects, ok = i.GetProjectsWritesProblem(c)
	}
	if !ok {
		return
	}

	for _, project := range projects.Value {
		projectInDB, ok := i.PutProjectWritesProblem(c, client, project)
		if !ok {
			fmt.Printf("Unable to import project %q", project.Name)
			return
		}

		ok = i.PostBranchesWritesProblem(c, client, project, projectInDB)
		if !ok {
			fmt.Printf("An error occured when importing branches from %q", project.Name)
			return
		}
	}

	c.Status(http.StatusCreated)
}

// prCreatedTriggerHandler godoc
// @Summary Triggers prcreated action on wharf-client
// @Accept json
// @Produce json
// @Param projectid path int true "wharf project ID"
// @Param azureDevOpsPR body azureDevOpsPR _ "AzureDevOps PR "
// @Param environment query string true "wharf build environment"
// @Success 200 {object} wharfapi.ProjectRunResponse "OK"
// @Failure 400 {object} problem.Response "Bad request"
// @Failure 401 {object} problem.Response "Unauthorized or missing jwt token"
// @Router /azuredevops/triggers/{projectid}/pr/created [post]
func prCreatedTriggerHandler(c *gin.Context) {
	const eventTypePullRequest string = "git.pullrequest.created"

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	t := azureDevOpsPR{}
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

	if err != nil {
		fmt.Println("Unable to send trigger to wharf-client: ", err)
		err = fmt.Errorf("unable to send trigger to wharf-client: %v", err)
		ginutil.WriteTriggerError(c, err, "Unable to send trigger to Wharf client.")
		return
	}

	c.JSON(http.StatusOK, resp)
}
