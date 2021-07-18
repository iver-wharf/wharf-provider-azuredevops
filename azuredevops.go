package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/iver-wharf/wharf-api-client-go/pkg/wharfapi"
	"github.com/iver-wharf/wharf-core/pkg/ginutil"
	"github.com/iver-wharf/wharf-core/pkg/problem"
	_ "github.com/iver-wharf/wharf-provider-azuredevops/docs"
	"github.com/iver-wharf/wharf-provider-azuredevops/pkg/requests"
)

const (
	apiRepositories = "_apis/git/repositories"
	apiProjects     = "_apis/projects"
	apiProviderName = "azuredevops"
	itemsPath       = "items"
	refsPath        = "refs"
)

type azureDevOpsCreator struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	URL         string `json:"url"`
	UniqueName  string `json:"uniqueName"`
	ImageURL    string `json:"imageUrl"`
	Descriptor  string `json:"descriptor"`
}

type azureDevOpsRef struct {
	ObjectID string             `json:"objectId"`
	Name     string             `json:"name"`
	Creator  azureDevOpsCreator `json:"creator"`
	URL      string             `json:"url"`
}

type azureDevOpsProject struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	State       string `json:"state"`
	Revision    int64  `json:"revision"`
	Visibility  string `json:"visibility"`
}

type azureDevOpsProjectResponse struct {
	Count int                  `json:"count"`
	Value []azureDevOpsProject `json:"value"`
}

type azureDevOpsRepository struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	URL           string             `json:"url"`
	Project       azureDevOpsProject `json:"project"`
	DefaultBranch string             `json:"defaultBranch"`
	Size          int64              `json:"size"`
	RemoteURL     string             `json:"remoteUrl"`
	SSHURL        string             `json:"sshUrl"`
}

type azureDevOpsRepositoryResponse struct {
	Count int                     `json:"count"`
	Value []azureDevOpsRepository `json:"value"`
}

type azureDevOpsBranch struct {
	Name          string
	Ref           string
	DefaultBranch bool
}

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

	i := importBody{}
	err := c.ShouldBindJSON(&i)
	if err != nil {
		ginutil.WriteInvalidBindError(c, err,
			"One or more parameters failed to parse when reading the request body for import details.")
			return
	}

	fmt.Println("from json: ", i)

	if i.Group == "" {
		fmt.Println("Unable to get due to empty group.")
		err := errors.New("missing required property: group")
		ginutil.WriteInvalidParamError(c, err, "group",
			"Unable to import due to empty group.")
		return
	}

	token, ok := getOrPostTokenWritesProblem(c, client, i)
	if !ok {
		fmt.Println("Unable to get or create token.")
		return
	}
	fmt.Println("Token from db: ", token)

	provider, ok := getOrPostProviderWritesProblem(c, client, token, i)
	if !ok {
		return
	}
	fmt.Println("Provider from db: ", provider)

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	var projects azureDevOpsProjectResponse
	if i.Project != "" {
		projects, ok = getProjectWritesProblem(c, i)
	} else {
		projects, ok = getProjectsWritesProblem(c, i)
	}
	if !ok {
		return
	}

	for _, project := range projects.Value {
		projectInDb, ok := putProjectWritesProblem(c, client, provider, i, project)
		if !ok {
			fmt.Printf("Unable to import project %q", project.Name)
			return
		}

		ok = postBranchesWritesProblem(c, client, i, project, projectInDb)
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

func putProjectWritesProblem(c *gin.Context, client wharfapi.Client, provider wharfapi.Provider,
	i importBody, project azureDevOpsProject) (wharfapi.Project, bool) {
	buildDefinitionStr, ok := getBuildDefinitionWritesProblem(c, i, project.Name)
	if !ok {
		return wharfapi.Project{}, false
	}

	gitURL, err := getGitURL(provider, i.Group, project)
	if err != nil {
		fmt.Println("Unable to construct git url ", err)
		ginutil.WriteComposingProviderDataError(c, err,
			fmt.Sprintf("Unable to construct git url for project '%s' in group '%s'", project.Name, i.Group))
		return wharfapi.Project{}, false
	}

	projectInDb, err := client.PutProject(
		wharfapi.Project{
			Name:            project.Name,
			TokenID:         i.TokenID,
			GroupName:       i.Group,
			BuildDefinition: buildDefinitionStr,
			Description:     project.Description,
			ProviderID:      provider.ProviderID,
			GitURL:          gitURL})

	if err != nil {
		fmt.Println("Unable to put project: ", err)
		ginutil.WriteAPIClientWriteError(c, err,
			fmt.Sprintf("Unable to import project '%s' from group '%s' at url '%s'.",
				i.Project, i.Group, gitURL))
		return wharfapi.Project{}, false
	}

	return projectInDb, true
}

func postBranchesWritesProblem(c *gin.Context, client wharfapi.Client, i importBody,
	project azureDevOpsProject, projectInDb wharfapi.Project) bool {
	repositories, ok := getRepositoriesWritesProblem(c, i, project)
	if !ok {
		return false
	}

	projectBranches, ok := getProjectBranchesWritesProblem(c, i, project.Name)
	if !ok {
		return false
	}

	for _, branch := range projectBranches {
		_, err := client.PutBranch(wharfapi.Branch{
			Name:      branch.Name,
			ProjectID: projectInDb.ProjectID,
			Default:   branch.Ref == repositories.Value[0].DefaultBranch,
			TokenID:   i.TokenID,
		})
		if err != nil {
			fmt.Println("Unable to post branch: ", err)
			ginutil.WriteAPIClientWriteError(c, err, fmt.Sprintf("Unable to import branch %q", branch.Name))
			return false
		}
	}

	return true
}

func getTokenByIDWritesProblem(c *gin.Context, client wharfapi.Client, tokenID uint) (wharfapi.Token, bool) {
	token, err := client.GetTokenById(tokenID)
	if err != nil || token.TokenID == 0 {
		fmt.Printf("Unable to get token. %+v", err)
		ginutil.WriteAPIClientReadError(c, err,
			fmt.Sprintf("Unable to get token by id %v.", tokenID))
		return wharfapi.Token{}, false
	}

	return token, true
}

func getOrPostTokenWritesProblem(c *gin.Context, client wharfapi.Client, i importBody) (wharfapi.Token, bool) {
	var err error
	if i.User == "" && i.TokenID == 0 {
		err = fmt.Errorf("both token and user were omitted")
		ginutil.WriteInvalidParamError(c, err, "user",
			"Unable to import when both user and token are omitted.")
		return wharfapi.Token{}, false
	}

	var token wharfapi.Token
	var ok bool
	if i.TokenID != 0 {
		token, ok = getTokenByIDWritesProblem(c, client, i.TokenID)
		if !ok {
			return wharfapi.Token{}, false
		}
	} else {
		token, err = client.GetToken(i.Token, i.User)
		if err != nil || token.TokenID == 0 {
			token, err = client.PostToken(wharfapi.Token{Token: i.Token, UserName: i.User})
			if err != nil {
				fmt.Println("Unable to post token: ", err)
				ginutil.WriteAPIClientWriteError(c, err, "Unable to get existing token or create new token.")
				return wharfapi.Token{}, false
			}
		}
	}

	i.User = token.UserName
	i.Token = token.Token
	i.TokenID = token.TokenID
	return token, true
}

func getOrPostProviderWritesProblem(c *gin.Context, client wharfapi.Client,
	token wharfapi.Token, i importBody) (wharfapi.Provider, bool) {
	var provider wharfapi.Provider
	if i.ProviderID != 0 {
		provider, err := client.GetProviderById(i.ProviderID)
		if err != nil || provider.ProviderID == 0 {
			fmt.Printf("Unable to get provider. %+v", err)
			ginutil.WriteAPIClientReadError(c, err,
				fmt.Sprintf("Unable to get provider by id %v", i.ProviderID))
			return wharfapi.Provider{}, false
		}
	} else {
		provider, err := client.GetProvider(apiProviderName, i.URL, i.UploadURL, token.TokenID)
		if err != nil || provider.ProviderID == 0 {
			provider, err = client.PostProvider(
				wharfapi.Provider{Name: apiProviderName, URL: i.URL, TokenID: token.TokenID})
			if err != nil {
				fmt.Println("Unable to post provider: ", err)
				ginutil.WriteAPIClientWriteError(c, err,
					fmt.Sprintf("Unable to get or create provider from '%s'.", i.URL))
				return wharfapi.Provider{}, false
			}
		}
	}

	i.URL = provider.URL
	return provider, true
}

func getRepositoriesWritesProblem(c *gin.Context, i importBody, project azureDevOpsProject) (azureDevOpsRepositoryResponse, bool) {
	const apiVersion string = "5.0"

	urlPath, err := requests.ConstructGetURL(i.URL, map[string][]string{
		"api-version": {apiVersion},
	}, "%v/%v/%v", i.Group, project.Name, apiRepositories)
	if err != nil {
		fmt.Println("Unable to get url: ", err)
		ginutil.WriteInvalidParamError(c, err, "URL", fmt.Sprintf("Unable to parse URL %q", i.URL))
		return azureDevOpsRepositoryResponse{}, false
	}

	fmt.Println(urlPath.String())

	repositories := azureDevOpsRepositoryResponse{}
	err = requests.GetAndParseJSON(&repositories, i.User, i.Token, urlPath)
	if err != nil {
		fmt.Println("Unable to get project repository: ", err)
		ginutil.WriteProviderResponseError(c, err, "Could be caused by invalid JSON data structure."+
			"\nMight be the result of an incompatible version of Azure DevOps.")
		return azureDevOpsRepositoryResponse{}, false
	}

	if repositories.Count != 1 {
		fmt.Println("One repository is required.")
		err = errors.New("one repository is required")
		ginutil.WriteAPIClientReadError(c, err,
			fmt.Sprintf("There were %d repositories, we need it to be 1.",
				repositories.Count))
		return azureDevOpsRepositoryResponse{}, false
	}

	if repositories.Value[0].Project.ID != project.ID {
		fmt.Println("Repository is not connected with project.")
		err = errors.New("repository is not connected with project")
		ginutil.WriteAPIClientReadError(c, err,
			fmt.Sprintf("Repository id (%s) and project id (%s) mismatch",
				repositories.Value[0].Project.ID,
				project.ID))
		return azureDevOpsRepositoryResponse{}, false
	}

	return repositories, true
}

func getBuildDefinitionWritesProblem(c *gin.Context, i importBody, projectName string) (string, bool) {
	urlPath, err := requests.ConstructGetURL(i.URL, map[string][]string{
		"scopePath": {fmt.Sprintf("/%v", buildDefinitionFileName)},
	}, "%s/%s/%s/%s/%s", i.Group, i.Project, apiRepositories, projectName, itemsPath)
	if err != nil {
		fmt.Println("Unable to get url: ", err)
		ginutil.WriteInvalidParamError(c, err, "URL", fmt.Sprintf("Unable to parse URL %q.", i.URL))
		return "", false
	}

	buildDefinitionStr, err := requests.GetAsString(i.User, i.Token, urlPath)
	if err != nil {
		fmt.Println("Unable to get build definition: ", err)
		ginutil.WriteFetchBuildDefinitionError(c, err,
			fmt.Sprintf("Unable to fetch build definition for %q.", i.Project))
		return "", false
	}

	return buildDefinitionStr, true
}

func getGitURL(provider wharfapi.Provider, group string, project azureDevOpsProject) (string, error) {
	providerURL, err := url.Parse(provider.URL)

	if err != nil {
		fmt.Println("Unable to parse provider url: ", err)
		return "", err
	}

	const sshPort = 22
	gitURL := fmt.Sprintf("git@%s:%d/%s/%s/_git/%s", providerURL.Host, sshPort, group, project.Name, project.Name)
	return gitURL, nil
}

func getProjectWritesProblem(c *gin.Context, i importBody) (azureDevOpsProjectResponse, bool) {
	const apiVersion string = "5.0"
	values := []interface{}{i.Group, apiProjects, i.Project}
	format := "%v/%v/%v"
	getProjectURL, err := requests.ConstructGetURL(i.URL, map[string][]string{
		"api-version": {apiVersion},
	}, format, values...)

	if err != nil {
		errorDetail := fmt.Sprintf("Unable to build url %q for '%v/%v/%v'",
			i.URL, values[0], values[1], values[2])

		ginutil.WriteInvalidParamError(c, err, "URL", errorDetail)
		return azureDevOpsProjectResponse{}, false
	}

	projects := azureDevOpsProjectResponse{
		Count: 1,
		Value: make([]azureDevOpsProject, 1),
	}

	err = requests.GetAndParseJSON(&projects.Value[0], i.User, i.Token, getProjectURL)

	if err != nil {
		ginutil.WriteProviderResponseError(c, err, "Could be caused by invalid JSON data structure."+
			"\nMight be the result of an incompatible version of Azure DevOps.")
		return azureDevOpsProjectResponse{}, false
	}

	return projects, true
}

func getProjectsWritesProblem(c *gin.Context, i importBody) (azureDevOpsProjectResponse, bool) {
	const apiVersion string = "5.0"
	values := []interface{}{i.Group, apiProjects}
	format := "%v/%v"
	getProjectsURL, err := requests.ConstructGetURL(i.URL, map[string][]string{
		"api-version": {apiVersion},
	}, format, values...)

	if err != nil {
		errorDetail := fmt.Sprintf("Unable to build url %q for '%v/%v'",
			i.URL, values[0], values[1])

		ginutil.WriteInvalidParamError(c, err, "URL", errorDetail)
		return azureDevOpsProjectResponse{}, false
	}

	projects := azureDevOpsProjectResponse{
		Count: 1,
		Value: make([]azureDevOpsProject, 1),
	}

	err = requests.GetAndParseJSON(&projects, i.User, i.Token, getProjectsURL)

	if err != nil {
		ginutil.WriteProviderResponseError(c, err, "Could be caused by invalid JSON data structure."+
			"\nMight be the result of an incompatible version of Azure DevOps.")
		return azureDevOpsProjectResponse{}, false
	}

	return projects, true
}

func getProjectBranchesWritesProblem(c *gin.Context, i importBody, project string) ([]azureDevOpsBranch, bool) {
	const apiVersion string = "5.0"

	urlPath, err := requests.ConstructGetURL(i.URL, map[string][]string{
		"api-version": {apiVersion},
		"filter":      {"heads/"},
	}, "%v/%v/%v/%v/%v", i.Group, project, apiRepositories, project, refsPath)

	if err != nil {
		ginutil.WriteInvalidParamError(c, err, "URL", fmt.Sprintf("Unable to parse URL %q", i.URL))
		return []azureDevOpsBranch{}, false
	}

	fmt.Println(urlPath.String())

	projectRefs := struct {
		Value []azureDevOpsRef `json:"value"`
		Count int              `json:"count"`
	}{}

	err = requests.GetAndParseJSON(&projectRefs, i.User, i.Token, urlPath)
	if err != nil {
		ginutil.WriteProviderResponseError(c, err, "Could be caused by invalid JSON data structure."+
			"\nMight be the result of an incompatible version of Azure DevOps.")
		return []azureDevOpsBranch{}, false
	}

	var projectBranches []azureDevOpsBranch
	for _, ref := range projectRefs.Value {
		name := strings.TrimPrefix(ref.Name, "refs/heads/")
		projectBranches = append(projectBranches, azureDevOpsBranch{
			Name: name,
			Ref:  ref.Name,
		})
	}

	return projectBranches, true
}
