package importer

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/iver-wharf/wharf-api-client-go/pkg/wharfapi"
	"github.com/iver-wharf/wharf-core/pkg/ginutil"
	"github.com/iver-wharf/wharf-provider-azuredevops/pkg/requests"
)

const (
	apiProviderName         = "azuredevops"
	buildDefinitionFileName = ".wharf-ci.yml"
	apiProjects             = "_apis/projects"
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

// AzureDevOpsProjectResponse is the response received from the remote provider
// when getting project(s).
type AzureDevOpsProjectResponse struct {
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

// AzureDevOpsImporter is used to communicate with the remote provider
// and the Wharf API.
type AzureDevOpsImporter struct {
	// used in refresh only
	TokenID     uint   `json:"tokenId" example:"0"`
	TokenString string `json:"token" example:"sample token"`
	UserName    string `json:"user" example:"sample user name"`
	URL         string `json:"url" example:"https://gitlab.local"`
	UploadURL   string `json:"uploadUrl" example:""`
	// used in refresh only
	ProviderID uint `json:"providerId" example:"0"`
	// azuredevops, gitlab or github
	ProviderName string `json:"provider" example:"gitlab"`
	// used in refresh only
	ProjectID   uint   `json:"projectId" example:"0"`
	ProjectName string `json:"project" example:"sample project name"`
	GroupName   string `json:"group" example:"default"`
	// retrieved from database
	Token wharfapi.Token `json:"-"`
	// retrieved from database
	Provider wharfapi.Provider `json:"-"`
}

// GetOrPostTokenWritesProblem attempts to get or create the token using the
// provided data.
//
// If a token ID is provided, it will attempt to retrieve the token from the
// database.
//
// If a username and token is provided, it will first attempt to retrieve the
// token from the database. If not successful, it will attempt to create a new
// token.
func (i *AzureDevOpsImporter) GetOrPostTokenWritesProblem(c *gin.Context, client wharfapi.Client) (wharfapi.Token, bool) {
	if i.UserName == "" && i.TokenID == 0 {
		err := errors.New("both token and user were omitted")
		ginutil.WriteInvalidParamError(c, err, "user",
			"Unable to import when both user and token are omitted.")
		return wharfapi.Token{}, false
	}

	var token wharfapi.Token
	if i.TokenID != 0 {
		var ok bool
		token, ok = i.getTokenByIDWritesProblem(c, client, i.TokenID)
		if !ok {
			return wharfapi.Token{}, false
		}
	} else {
		var err error
		token, err = client.GetToken(i.TokenString, i.UserName)
		if err != nil || token.TokenID == 0 {
			token, err = client.PostToken(wharfapi.Token{Token: i.TokenString, UserName: i.UserName})
			if err != nil {
				fmt.Println("Unable to post token: ", err)
				ginutil.WriteAPIClientWriteError(c, err, "Unable to get existing token or create new token.")
				return wharfapi.Token{}, false
			}
		}
	}

	return token, true
}

// GetOrPostProviderWritesProblem attempts to get or create the provider using
// the provided data.
//
// If a provider ID is provided, it will attempt to retrieve the provider from the
// database.
//
// Otherwise, it will try to get a provider from the database matching the
// provided data. If not successful, it will attempt to create a new provider.
func (i *AzureDevOpsImporter) GetOrPostProviderWritesProblem(c *gin.Context, client wharfapi.Client) (wharfapi.Provider, bool) {
	var provider wharfapi.Provider
	if i.Provider.ProviderID != 0 {
		provider, err := client.GetProviderById(i.Provider.ProviderID)
		if err != nil || provider.ProviderID == 0 {
			fmt.Printf("Unable to get provider. %+v", err)
			ginutil.WriteAPIClientReadError(c, err,
				fmt.Sprintf("Unable to get provider by ID %d", i.Provider.ProviderID))
			return wharfapi.Provider{}, false
		}
	} else {
		var err error
		provider, err = client.GetProvider(apiProviderName, i.URL, i.UploadURL, i.Token.TokenID)
		if err != nil || provider.ProviderID == 0 {
			provider, err = client.PostProvider(
				wharfapi.Provider{Name: apiProviderName, URL: i.URL, TokenID: i.Token.TokenID})
			if err != nil {
				fmt.Println("Unable to post provider: ", err)
				ginutil.WriteAPIClientWriteError(c, err,
					fmt.Sprintf("Unable to get or create provider from '%s'.", i.URL))
				return wharfapi.Provider{}, false
			}
		}
	}

	return provider, true
}

// GetProjectWritesProblem attempts to get a project from the specified URL,
// matching the provided group and project name.
func (i *AzureDevOpsImporter) GetProjectWritesProblem(c *gin.Context) (AzureDevOpsProjectResponse, bool) {
	getProjectURL, err := requests.NewGetProject(i.URL, i.GroupName, i.ProjectName)

	if err != nil {
		errorDetail := fmt.Sprintf("Unable to build url %q for '%s/%s/%s'",
			i.URL, i.GroupName, apiProjects, i.ProjectName)

		ginutil.WriteInvalidParamError(c, err, "url", errorDetail)
		return AzureDevOpsProjectResponse{}, false
	}

	projects := AzureDevOpsProjectResponse{
		Count: 1,
		Value: make([]azureDevOpsProject, 1),
	}

	err = requests.GetAndParseJSON(&projects.Value[0], i.UserName, i.Token.Token, getProjectURL)

	if err != nil {
		ginutil.WriteProviderResponseError(c, err,
			"Invalid response when getting project. "+
				"Could be caused by invalid JSON data structure. "+
				"Might be the result of an incompatible version of Azure DevOps.")
		return AzureDevOpsProjectResponse{}, false
	}

	return projects, true
}

// GetProjectsWritesProblem attempts to get all projects from the specified URL
// that are part of the provided group.
func (i *AzureDevOpsImporter) GetProjectsWritesProblem(c *gin.Context) (AzureDevOpsProjectResponse, bool) {
	getProjectsURL, err := requests.NewGetProjects(i.URL, i.GroupName)

	if err != nil {
		errorDetail := fmt.Sprintf("Unable to build url %q for '%s/%s'",
			i.URL, i.GroupName, apiProjects)

		ginutil.WriteInvalidParamError(c, err, "URL", errorDetail)
		return AzureDevOpsProjectResponse{}, false
	}

	projects := AzureDevOpsProjectResponse{
		Count: 1,
		Value: make([]azureDevOpsProject, 1),
	}

	err = requests.GetAndParseJSON(&projects, i.UserName, i.Token.Token, getProjectsURL)

	if err != nil {
		ginutil.WriteProviderResponseError(c, err, "Could be caused by invalid JSON data structure."+
			"\nMight be the result of an incompatible version of Azure DevOps.")
		return AzureDevOpsProjectResponse{}, false
	}

	return projects, true
}

// PutProjectWritesProblem attempts to insert a project into the Wharf database.
func (i *AzureDevOpsImporter) PutProjectWritesProblem(c *gin.Context, client wharfapi.Client, project azureDevOpsProject) (wharfapi.Project, bool) {
	buildDefinitionStr, ok := i.getBuildDefinitionWritesProblem(c, project.Name)
	if !ok {
		return wharfapi.Project{}, false
	}

	gitURL, err := i.getGitURL(project)
	if err != nil {
		fmt.Println("Unable to construct git url ", err)
		ginutil.WriteComposingProviderDataError(c, err,
			fmt.Sprintf("Unable to construct git url for project '%s' in group '%s'", project.Name, i.GroupName))
		return wharfapi.Project{}, false
	}

	projectInDB, err := client.PutProject(wharfapi.Project{
		Name:            project.Name,
		TokenID:         i.Token.TokenID,
		GroupName:       i.GroupName,
		BuildDefinition: buildDefinitionStr,
		Description:     project.Description,
		ProviderID:      i.Provider.ProviderID,
		GitURL:          gitURL})

	if err != nil {
		fmt.Println("Unable to put project: ", err)
		ginutil.WriteAPIClientWriteError(c, err,
			fmt.Sprintf("Unable to import project '%s' from group '%s' at url '%s'.",
				i.ProjectName, i.GroupName, gitURL))
		return wharfapi.Project{}, false
	}

	return projectInDB, true
}

// PostBranchesWritesProblem attempts to insert a project's branches into the
// Wharf database.
func (i *AzureDevOpsImporter) PostBranchesWritesProblem(c *gin.Context, client wharfapi.Client,
	project azureDevOpsProject, projectInDB wharfapi.Project) bool {
	repositories, ok := i.getRepositoriesWritesProblem(c, project)
	if !ok {
		return false
	}

	projectBranches, ok := i.getProjectBranchesWritesProblem(c, project)
	if !ok {
		return false
	}

	for _, branch := range projectBranches {
		_, err := client.PutBranch(wharfapi.Branch{
			Name:      branch.Name,
			ProjectID: projectInDB.ProjectID,
			Default:   branch.Ref == repositories.Value[0].DefaultBranch,
			TokenID:   i.Token.TokenID,
		})
		if err != nil {
			fmt.Println("Unable to post branch: ", err)
			ginutil.WriteAPIClientWriteError(c, err, fmt.Sprintf("Unable to import branch %q", branch.Name))
			return false
		}
	}

	return true
}

func (i *AzureDevOpsImporter) getTokenByIDWritesProblem(c *gin.Context, client wharfapi.Client, tokenID uint) (wharfapi.Token, bool) {
	token, err := client.GetTokenById(tokenID)
	if err != nil || token.TokenID == 0 {
		fmt.Printf("Unable to get token. %+v", err)
		ginutil.WriteAPIClientReadError(c, err,
			fmt.Sprintf("Unable to get token by ID %d.", tokenID))
		return wharfapi.Token{}, false
	}

	return token, true
}

func (i *AzureDevOpsImporter) getRepositoriesWritesProblem(c *gin.Context, project azureDevOpsProject) (azureDevOpsRepositoryResponse, bool) {
	urlPath, err := requests.NewGetRepositories(i.URL, i.GroupName, i.ProjectName)
	if err != nil {
		fmt.Println("Unable to get url: ", err)
		ginutil.WriteInvalidParamError(c, err, "URL", fmt.Sprintf("Unable to parse URL %q", i.URL))
		return azureDevOpsRepositoryResponse{}, false
	}

	fmt.Println(urlPath.String())

	repositories := azureDevOpsRepositoryResponse{}
	err = requests.GetAndParseJSON(&repositories, i.UserName, i.Token.Token, urlPath)
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
			fmt.Sprintf("Repository ID (%s) and project ID (%s) mismatch.",
				repositories.Value[0].Project.ID,
				project.ID))
		return azureDevOpsRepositoryResponse{}, false
	}

	return repositories, true
}

func (i *AzureDevOpsImporter) getBuildDefinitionWritesProblem(c *gin.Context, projectName string) (string, bool) {
	urlPath, err := requests.NewGetFile(i.URL, i.GroupName, projectName, buildDefinitionFileName)
	if err != nil {
		fmt.Println("Unable to get url: ", err)
		ginutil.WriteInvalidParamError(c, err, "url", fmt.Sprintf("Unable to parse URL %q.", i.URL))
		return "", false
	}

	buildDefinitionStr, err := requests.GetAsString(i.UserName, i.Token.Token, urlPath)
	if err != nil {
		fmt.Println("Unable to get build definition: ", err)
		ginutil.WriteFetchBuildDefinitionError(c, err,
			fmt.Sprintf("Unable to fetch build definition for project %q.", i.ProjectName))
		return "", false
	}

	return buildDefinitionStr, true
}

func (i *AzureDevOpsImporter) getGitURL(project azureDevOpsProject) (string, error) {
	providerURL, err := url.Parse(i.Provider.URL)

	if err != nil {
		fmt.Println("Unable to parse provider url: ", err)
		return "", err
	}

	const sshPort = 22
	gitURL := fmt.Sprintf("git@%s:%d/%s/%s/_git/%s", providerURL.Host, sshPort, i.GroupName, project.Name, project.Name)
	return gitURL, nil
}

func (i *AzureDevOpsImporter) getProjectBranchesWritesProblem(c *gin.Context, project azureDevOpsProject) ([]azureDevOpsBranch, bool) {
	urlPath, err := requests.NewGetGitRefs(i.URL, i.GroupName, project.Name, "heads/")
	if err != nil {
		ginutil.WriteInvalidParamError(c, err, "URL", fmt.Sprintf("Unable to parse URL %q", i.URL))
		return []azureDevOpsBranch{}, false
	}

	fmt.Println(urlPath.String())

	projectRefs := struct {
		Value []azureDevOpsRef `json:"value"`
		Count int              `json:"count"`
	}{}

	err = requests.GetAndParseJSON(&projectRefs, i.UserName, i.Token.Token, urlPath)
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
