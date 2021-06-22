package main

import (
	"crypto/tls"
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
	"github.com/iver-wharf/wharf-provider-azuredevops/helpers/ginutilext"
	"github.com/iver-wharf/wharf-provider-azuredevops/helpers/requests"
	"github.com/iver-wharf/wharf-provider-azuredevops/helpers/wharfapiext"
)

const apiRepositories string = "_apis/git/repositories"
const apiProjects string = "_apis/projects"
const apiProviderName string = "azuredevops"
const apiVersion string = "5.0"
const eventTypePullRequest string = "git.pullrequest.created"
const itemsPath string = "items"
const refsPath string = "refs"

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

type azureDevOpsProjectSlice struct {
	Count int `json:"count"`
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

type azureDevOpsRepositorySlice struct {
	Count int `json:"count"`
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

	client := wharfapiext.ExtClient{Client: &wharfapi.Client{
		ApiUrl:     os.Getenv("WHARF_API_URL"),
		AuthHeader: c.GetHeader("Authorization"),
	}}

	i, err := bindImportDetails(c)
	if err != nil {
		ginutil.WriteInvalidBindError(c, err,
			"One or more parameters failed to parse when reading the request body for import details.")
		return
	}
	fmt.Println("from json: ", i)

	if i.Group == "" {
		fmt.Println("Unable to get due to empty group.")
		ginutil.WriteInvalidParamError(c, err, "group",
			"Unable to import due to empty group.")
		return
	}

	token, err := getOrPostTokenWritesError(c, client, i)
	if err != nil {
		fmt.Println("Unable to get or create token.")
		return
	}
	fmt.Println("Token from db: ", token)

	provider, err := getOrPostProviderWritesError(c, client, token, i)
	if err != nil {
		return
	}
	fmt.Println("Provider from db: ", provider)

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	projects, err := getProjectsWritesError(c, i)
	if err != nil {
		return
	}

	for _, project := range projects.Value {
		projectInDb, err := tryPutProjectWritesError(c, client, provider, i, project)
		if err != nil {
			fmt.Printf("Unable to import project %q", project.Name)
			return
		}

		err = tryPostBranchesWritesError(c, client, i, project, projectInDb)
		if err != nil {
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
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	t := azureDevOpsPR{}
	if err := c.BindJSON(&t); err != nil {
		ginutil.WriteInvalidBindError(c, err,
			"One or more parameters failed to parse when reading the request body for pull request.")
		return
	}

	if t.EventType != eventTypePullRequest {
		err := fmt.Errorf("expected event type %q for trigger, got: %q", eventTypePullRequest, t.EventType)
		ginutil.WriteProblemError(c, err, problem.Response{
			Type: "/prob/provider/azuredevops/unsupported-event-type",
			Title: "Invalid event type.",
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
		ginutilext.WriteTriggerError(c, err, "Unable to send trigger to Wharf client.")
		return
	}

	c.JSON(http.StatusOK, resp)
}

func bindImportDetails(c *gin.Context) (importBody, error) {
	i := importBody{}
	err := c.BindJSON(&i)
	if err != nil {
		return importBody{}, err
	}

	return i, nil
}

// tryPutProjectWritesError Writes an error to the gin Context when an error occurs!
func tryPutProjectWritesError(c *gin.Context, client wharfapiext.ExtClient, provider wharfapi.Provider,
	i importBody, project azureDevOpsProject) (wharfapi.Project, error) {
	buildDefinitionStr, err := getBuildDefinitionWritesError(c, i, project.Name)
	if err != nil {
		return wharfapi.Project{}, err
	}

	gitURL, err := getGitURL(provider, i.Group, project)
	if err != nil {
		fmt.Println("Unable to construct git url ", err)
		ginutilext.WriteComposingProviderDataError(c, err,
			fmt.Sprintf("Unable to construct git url for project '%s' in group '%s'", project.Name, i.Group))
		return wharfapi.Project{}, err
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
		ginutilext.WriteAPIWriteError(c, err,
			fmt.Sprintf("Unable to import project '%s' from group '%s' at url '%s'.",
				i.Project, i.Group, gitURL))
		return wharfapi.Project{}, err
	}

	return projectInDb, nil
}

// tryPostBranchesWritesError Writes an error to the gin Context when an error occurs!
func tryPostBranchesWritesError(c *gin.Context, client wharfapiext.ExtClient, i importBody,
	project azureDevOpsProject, projectInDb wharfapi.Project) error {
	repositories, err := getRepositoriesWritesError(c, i, project)
	if err != nil {
		fmt.Println("Unable to get repos: ", err)
		return err
	}

	projectBranches, err := getProjectBranchesWritesError(c, i, project.Name)
	if err != nil {
		fmt.Println("Unable to get project branches: ", err)
		return err
	}

	for _, branch := range projectBranches {
		_, err := client.PostBranch(wharfapi.Branch{
			Name:      branch.Name,
			ProjectID: projectInDb.ProjectID,
			Default:   branch.Ref == repositories.Value[0].DefaultBranch,
			TokenID:   i.TokenID,
		})
		if err != nil {
			fmt.Println("Unable to post branch: ", err)
			ginutilext.WriteAPIWriteError(c, err, fmt.Sprintf("Unable to import branch '%s'", branch.Name))
			return err
		}
	}

	return nil
}

// getTokenByIDWritesError Writes an error to the gin Context when an error occurs!
func getTokenByIDWritesError(c *gin.Context, client wharfapiext.ExtClient, tokenID uint) (wharfapi.Token, error) {
	token, err := client.GetTokenById(tokenID)
	if err != nil || token.TokenID == 0 {
		fmt.Printf("Unable to get token. %+v", err)
		ginutilext.WriteAPIReadError(c, err,
			fmt.Sprintf("Unable to get token by id %v.", tokenID))
		return wharfapi.Token{}, err
	}

	return token, nil
}

// getOrPostTokenWritesError Writes an error to the gin Context when an error occurs!
func getOrPostTokenWritesError(c *gin.Context, client wharfapiext.ExtClient, i importBody) (wharfapi.Token, error) {
	var err error
	if i.User == "" && i.TokenID == 0 {
		err = fmt.Errorf("both token and user were omitted")
		ginutil.WriteInvalidParamError(c, err, "user",
			"Unable to import when both user and token are omitted.")
		return wharfapi.Token{}, err
	}

	var token wharfapi.Token
	if i.TokenID != 0 {
		token, err = getTokenByIDWritesError(c, client, i.TokenID)
		if err != nil {
			ginutilext.WriteAPIReadError(c, err,
				fmt.Sprintf("Unable to get token by id %v.", i.TokenID))
			return wharfapi.Token{}, err
		}
	} else {
		token, err = client.GetToken(i.Token, i.User)
		if err != nil || token.TokenID == 0 {
			token, err = client.PostToken(wharfapi.Token{Token: i.Token, UserName: i.User})
			if err != nil {
				fmt.Println("Unable to post token: ", err)
				ginutilext.WriteAPIWriteError(c, err,"Unable to get existing token or create new token.")
				return wharfapi.Token{}, err
			}
		}
	}

	i.User = token.UserName
	i.Token = token.Token
	i.TokenID = token.TokenID
	return token, nil
}

// getOrPostProviderWritesError Writes an error to the gin Context when an error occurs!
func getOrPostProviderWritesError(c *gin.Context, client wharfapiext.ExtClient,
	token wharfapi.Token, i importBody) (wharfapi.Provider, error) {
	var provider wharfapi.Provider
	if i.ProviderID != 0 {
		provider, err := client.GetProviderById(i.ProviderID)
		if err != nil || provider.ProviderID == 0 {
			fmt.Printf("Unable to get provider. %+v", err)
			ginutilext.WriteAPIReadError(c, err,
				fmt.Sprintf("Unable to get provider by id %v", i.ProviderID))
			return wharfapi.Provider{}, err
		}
	} else {
		provider, err := client.GetProvider(apiProviderName, i.URL, i.UploadURL, token.TokenID)
		if err != nil || provider.ProviderID == 0 {
			provider, err = client.PostProvider(
				wharfapi.Provider{Name: apiProviderName, URL: i.URL, TokenID: token.TokenID})
			if err != nil {
				fmt.Println("Unable to post provider: ", err)
				ginutilext.WriteAPIWriteError(c, err,
					fmt.Sprintf("Unable to get or create provider from '%s'.", i.URL))
				return wharfapi.Provider{}, err
			}
		}
	}

	i.URL = provider.URL
	return provider, nil
}

// getRepositoriesWritesError Writes an error to the gin Context when an error occurs!
func getRepositoriesWritesError(c *gin.Context, i importBody, project azureDevOpsProject) (azureDevOpsRepositorySlice, error) {
	urlPath, err := requests.ConstructGetURL(i.URL, map[string][]string{
		"api-version": {apiVersion},
	}, "%v/%v/%v", i.Group, project.Name, apiRepositories)
	if err != nil {
		fmt.Println("Unable to get url: ", err)
		ginutil.WriteInvalidParamError(c, err, "URL", fmt.Sprintf("Unable to parse URL %q", i.URL))
		return azureDevOpsRepositorySlice{}, err
	}

	fmt.Println(urlPath.String())

	repositories := azureDevOpsRepositorySlice{}
	err = requests.GetAndParseJSON(&repositories, i.User, i.Token, urlPath)
	if err != nil {
		fmt.Println("Unable to get project repository: ", err)
		ginutilext.WriteAPIReadError(c, err,
			fmt.Sprintf("Unable to fetch project repository from project '%s'.\nurl: %q",
				project.Name,
				urlPath.String()))
		return azureDevOpsRepositorySlice{}, err
	}

	if repositories.Count != 1 {
		fmt.Println("One repository is required.")
		ginutilext.WriteAPIReadError(c, err,
			fmt.Sprintf("There were %v repositories, we need it to be 1.",
				repositories.Count))
		return azureDevOpsRepositorySlice{}, fmt.Errorf("one repository is required")
	}

	if repositories.Value[0].Project.ID != project.ID {
		fmt.Println("Repository is not connected with project.")
		ginutilext.WriteAPIReadError(c, err,
			fmt.Sprintf("Repository id (%v) and project id (%v) mismatch",
				repositories.Value[0].Project.ID,
				project.ID))
		return azureDevOpsRepositorySlice{}, fmt.Errorf("repository is not connected with project")
	}

	return repositories, nil
}

// getBuildDefinitionWritesError Writes an error to the gin Context when an error occurs!
func getBuildDefinitionWritesError(c *gin.Context, i importBody, projectName string) (string, error) {
	urlPath, err := requests.ConstructGetURL(i.URL, map[string][]string{
		"scopePath": {fmt.Sprintf("/%v", buildDefinitionFileName)},
	}, "%v/%v/%v/%v/%v", i.Group, i.Project, apiRepositories, projectName, itemsPath)
	if err != nil {
		fmt.Println("Unable to get url: ", err)
		ginutil.WriteInvalidParamError(c, err, "URL", fmt.Sprintf("Unable to parse URL %q", i.URL))
		return "", err
	}

	buildDefinitionStr, err := requests.GetAsString(i.User, i.Token, urlPath)
	if err != nil {
		fmt.Println("Unable to get build definition: ", err)
		ginutilext.WriteFetchBuildDefinitionError(c, err,
			fmt.Sprintf("Unable to fetch build definition for '%s'", i.Project))
		return "", err
	}

	return buildDefinitionStr, nil
}

func getGitURL(provider wharfapi.Provider, group string, project azureDevOpsProject) (string, error) {
	providerURL, err := url.Parse(provider.URL)

	if err != nil {
		fmt.Println("Unable to parse provider url: ", err)
		return "", err
	}

	gitURL := fmt.Sprintf("git@%v:22/%v/%v/_git/%v", providerURL.Host, group, project.Name, project.Name)
	return gitURL, nil
}

// getProjectsWritesError Writes an error to the gin Context when an error occurs!
func getProjectsWritesError(c *gin.Context, i importBody) (azureDevOpsProjectSlice, error) {
	var getProjectsURL *url.URL
	var format string
	var values []interface{}
	if i.Project != "" {
		format = "%v/%v/%v"
		values = []interface{}{ i.Group, apiProjects, i.Project }
	} else {
		format = "%v/%v"
		values = []interface{}{ i.Group, apiProjects }
	}

	getProjectsURL, err := requests.ConstructGetURL(i.URL, map[string][]string{
		"api-version": {apiVersion},
	}, format, values...)

	if err != nil {
		var errorDetail string

		if i.Project != "" {
			errorDetail = fmt.Sprintf("Unable to build url '%v' for '%v/%v/%v'",
				i.URL, values[0], values[1], values[2])
		} else {
			errorDetail = fmt.Sprintf("Unable to build url '%v' for '%v/%v'",
				i.URL, values[0], values[1])
		}

		ginutil.WriteInvalidParamError(c, err, "URL", errorDetail)
		return azureDevOpsProjectSlice{}, err
	}

	projects := azureDevOpsProjectSlice{
		Count: 1,
		Value: make([]azureDevOpsProject, 1),
	}

	if i.Project != "" {
		err = requests.GetAndParseJSON(&projects.Value[0], i.User, i.Token, getProjectsURL)
	} else {
		err = requests.GetAndParseJSON(&projects, i.User, i.Token, getProjectsURL)
	}

	if err != nil {
		ginutilext.WriteResponseFormatError(c, err, "Could be caused by invalid JSON data structure." +
			"\nMight be the result of an incompatible version of Azure DevOps.")
		return azureDevOpsProjectSlice{}, err
	}

	return projects, nil
}

// getProjectBranchesWritesError Writes an error to the gin Context when an error occurs!
func getProjectBranchesWritesError(c *gin.Context, i importBody, project string) ([]azureDevOpsBranch, error) {
	urlPath, err := requests.ConstructGetURL(i.URL, map[string][]string{
		"api-version": {apiVersion},
		"filter": {"heads/"},
	}, "%v/%v/%v/%v/%v", i.Group, project, apiRepositories, project, refsPath)

	if err != nil {
		ginutil.WriteInvalidParamError(c, err, "URL", fmt.Sprintf("Unable to parse URL %q", i.URL))
		return []azureDevOpsBranch{}, err
	}

	fmt.Println(urlPath.String())

	projectRefs := struct {
		Value []azureDevOpsRef `json:"value"`
		Count int              `json:"count"`
	}{}

	err = requests.GetAndParseJSON(&projectRefs, i.User, i.Token, urlPath)
	if err != nil {
		ginutilext.WriteAPIReadError(c, err,
			fmt.Sprintf("Unable to get or parse JSON response from Azure DevOps API: %q.\n", urlPath.String()))
		return []azureDevOpsBranch{}, err
	}

	var projectBranches []azureDevOpsBranch
	for _, ref := range projectRefs.Value {
		name := strings.TrimPrefix(ref.Name, "refs/heads/")
		projectBranches = append(projectBranches, azureDevOpsBranch{
			Name: name,
			Ref:  ref.Name,
		})
	}

	return projectBranches, nil
}