package azure

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

type azureClient struct {
	WharfClient wharfapi.Client
	context     *gin.Context
	Token       wharfapi.Token
	Provider    wharfapi.Provider
	// used in refresh only
	UserName  string
	URL       string
	UploadURL string
}

// GetProjectWritesProblem attempts to get a project from the remote provider,
// matching the provided group and project name.
func (c *azureClient) GetProjectWritesProblem(groupName, projectName string) (azureProjectResponse, bool) {
	getProjectURL, err := requests.NewGetProject(c.URL, groupName, projectName)

	if err != nil {
		errorDetail := fmt.Sprintf("Unable to build url %q for '%s/%s/%s'",
			c.URL, groupName, apiProjects, projectName)

		ginutil.WriteInvalidParamError(c.context, err, "url", errorDetail)
		return azureProjectResponse{}, false
	}

	projects := azureProjectResponse{
		Count: 1,
		Value: make([]azureProject, 1),
	}

	err = requests.GetAndParseJSON(&projects.Value[0], c.UserName, c.Token.Token, getProjectURL)

	if err != nil {
		ginutil.WriteProviderResponseError(c.context, err,
			fmt.Sprintf("Invalid response when getting project %q from group %q. ", projectName, groupName)+
				"Could be caused by invalid JSON data structure. "+
				"Might be the result of an incompatible version of Azure DevOps.")
		return azureProjectResponse{}, false
	}

	return projects, true
}

// GetProjectsWritesProblem attempts to get all projects from the specified URL
// that are part of the provided group.
func (c *azureClient) GetProjectsWritesProblem(groupName string) (azureProjectResponse, bool) {
	getProjectsURL, err := requests.NewGetProjects(c.URL, groupName)

	if err != nil {
		errorDetail := fmt.Sprintf("Unable to build url %q for '%s/%s'",
			c.URL, groupName, apiProjects)

		ginutil.WriteInvalidParamError(c.context, err, "URL", errorDetail)
		return azureProjectResponse{}, false
	}

	projects := azureProjectResponse{
		Count: 1,
		Value: make([]azureProject, 1),
	}

	err = requests.GetAndParseJSON(&projects, c.UserName, c.Token.Token, getProjectsURL)

	if err != nil {
		ginutil.WriteProviderResponseError(c.context, err,
			fmt.Sprintf("Invalid response getting projects from group %q. ", groupName)+
				"Could be caused by invalid JSON data structure. "+
				"Might be the result of an incompatible version of Azure DevOps.")
		return azureProjectResponse{}, false
	}

	return projects, true
}

func (c *azureClient) getRepositoriesWritesProblem(groupName string, project azureProject) (azureRepositoryResponse, bool) {
	urlPath, err := requests.NewGetRepositories(c.URL, groupName, project.Name)
	if err != nil {
		fmt.Println("Unable to get url: ", err)
		ginutil.WriteInvalidParamError(c.context, err, "URL", fmt.Sprintf("Unable to parse URL %q", c.URL))
		return azureRepositoryResponse{}, false
	}

	fmt.Println(urlPath.String())

	repositories := azureRepositoryResponse{}
	err = requests.GetAndParseJSON(&repositories, c.UserName, c.Token.Token, urlPath)
	if err != nil {
		fmt.Println("Unable to get project repository: ", err)
		ginutil.WriteProviderResponseError(c.context, err,
			fmt.Sprintf(
				"Invalid response getting repositories from project %q in group %q. ",
				project.Name, groupName)+
				"Could be caused by invalid JSON data structure. "+
				"Might be the result of an incompatible version of Azure DevOps.")
		return azureRepositoryResponse{}, false
	}

	if repositories.Count != 1 {
		fmt.Println("One repository is required.")
		err = errors.New("one repository is required")
		ginutil.WriteAPIClientReadError(c.context, err,
			fmt.Sprintf("There were %d repositories, we need it to be 1.",
				repositories.Count))
		return azureRepositoryResponse{}, false
	}

	if repositories.Value[0].Project.ID != project.ID {
		fmt.Println("Repository is not connected with project.")
		err = errors.New("repository is not connected with project")
		ginutil.WriteAPIClientReadError(c.context, err,
			fmt.Sprintf("Repository ID (%s) and project ID (%s) mismatch.",
				repositories.Value[0].Project.ID,
				project.ID))
		return azureRepositoryResponse{}, false
	}

	return repositories, true
}

func (c *azureClient) getBuildDefinitionWritesProblem(groupName, projectName, filePath string) (string, bool) {
	urlPath, err := requests.NewGetFile(c.URL, groupName, projectName, filePath)
	if err != nil {
		fmt.Println("Unable to get url: ", err)
		ginutil.WriteInvalidParamError(c.context, err, "url", fmt.Sprintf("Unable to parse URL %q.", c.URL))
		return "", false
	}

	buildDefinitionStr, err := requests.GetAsString(c.UserName, c.Token.Token, urlPath)
	if err != nil {
		fmt.Println("Unable to get build definition: ", err)
		ginutil.WriteFetchBuildDefinitionError(c.context, err,
			fmt.Sprintf("Unable to fetch build definition for project %q.", projectName))
		return "", false
	}

	return buildDefinitionStr, true
}

func (c *azureClient) getGitURL(providerRawURL, groupName, projectName string) (string, error) {
	providerURL, err := url.Parse(providerRawURL)

	if err != nil {
		fmt.Println("Unable to parse provider url: ", err)
		return "", err
	}

	const sshPort = 22
	gitURL := fmt.Sprintf("git@%s:%d/%s/%s/_git/%s", providerURL.Host, sshPort, groupName, projectName, projectName)
	return gitURL, nil
}

func (c *azureClient) getProjectBranchesWritesProblem(groupName, projectName, refsFilter string) ([]azureBranch, bool) {
	urlPath, err := requests.NewGetGitRefs(c.URL, groupName, projectName, refsFilter)
	if err != nil {
		ginutil.WriteInvalidParamError(c.context, err, "URL", fmt.Sprintf("Unable to parse URL %q", c.URL))
		return []azureBranch{}, false
	}

	fmt.Println(urlPath.String())

	projectRefs := struct {
		Value []azureRef `json:"value"`
		Count int        `json:"count"`
	}{}

	err = requests.GetAndParseJSON(&projectRefs, c.UserName, c.Token.Token, urlPath)
	if err != nil {
		ginutil.WriteProviderResponseError(c.context, err,
			fmt.Sprintf(
				"Invalid response getting branches for project %q in group %q, using refs filter %q. ",
				projectName, groupName, refsFilter)+
				"Could be caused by invalid JSON data structure. "+
				"Might be the result of an incompatible version of Azure DevOps.")
		return []azureBranch{}, false
	}

	var projectBranches []azureBranch
	refsFilter = fmt.Sprintf("refs/%s", refsFilter)
	for _, ref := range projectRefs.Value {
		name := strings.TrimPrefix(ref.Name, refsFilter)
		projectBranches = append(projectBranches, azureBranch{
			Name: name,
			Ref:  ref.Name,
		})
	}

	return projectBranches, true
}
