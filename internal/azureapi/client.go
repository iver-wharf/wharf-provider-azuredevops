package azureapi

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/iver-wharf/wharf-core/pkg/ginutil"
	"github.com/iver-wharf/wharf-core/pkg/logger"
	"github.com/iver-wharf/wharf-provider-azuredevops/pkg/requests"
)

var log = logger.NewScoped("AZURE-API")

// Client is used to talk with the Azure DevOps API.
type Client struct {
	Context *gin.Context
	BaseURL string
	// BaseURLParsed is the result of url.Parse(BaseURL)
	BaseURLParsed *url.URL
	UserName      string
	Token         string
}

// GetProjectWritesProblem attempts to get a project from the remote provider,
// matching the provided group and project name.
func (c *Client) GetProjectWritesProblem(groupName, projectName string) (Project, bool) {
	getProjectURL, err := c.newGetProject(groupName, projectName)

	if err != nil {
		errorDetail := fmt.Sprintf("Unable to build url %q for '%s/_apis/projects/%s'",
			c.BaseURL, groupName, projectName)

		ginutil.WriteInvalidParamError(c.Context, err, "url", errorDetail)
		return Project{}, false
	}

	projects := projectResponse{
		Count: 1,
		Value: make([]Project, 1),
	}

	err = requests.GetUnmarshalJSON(&projects.Value[0], c.UserName, c.Token, getProjectURL)

	if err != nil {
		ginutil.WriteProviderResponseError(c.Context, err,
			fmt.Sprintf("Invalid response when getting project %q from group %q. ", projectName, groupName)+
				"Could be caused by invalid JSON data structure. "+
				"Might be the result of an incompatible version of Azure DevOps.")
		return Project{}, false
	}

	return projects.Value[0], true
}

// GetProjectsWritesProblem attempts to get all projects from the specified URL
// that are part of the provided group.
func (c *Client) GetProjectsWritesProblem(groupName string) ([]Project, bool) {
	getProjectsURL, err := c.newGetProjects(groupName)

	if err != nil {
		errorDetail := fmt.Sprintf("Unable to build url %q for '%s/_apis/projects'",
			c.BaseURL, groupName)

		ginutil.WriteInvalidParamError(c.Context, err, "URL", errorDetail)
		return []Project{}, false
	}

	projects := projectResponse{
		Count: 1,
		Value: make([]Project, 1),
	}

	err = requests.GetUnmarshalJSON(&projects, c.UserName, c.Token, getProjectsURL)
	if err != nil {
		ginutil.WriteProviderResponseError(c.Context, err,
			fmt.Sprintf("Invalid response getting projects from group %q. ", groupName)+
				"Could be caused by invalid JSON data structure. "+
				"Might be the result of an incompatible version of Azure DevOps.")
		return []Project{}, false
	}

	return projects.Value, true
}

// GetRepositoryWritesProblem attempts to get a repository matching the
// specified project's id using BasicAuth.
func (c *Client) GetRepositoryWritesProblem(groupName string, project Project) (Repository, bool) {
	urlPath, err := c.newGetRepositories(groupName, project.Name)
	if err != nil {
		log.Error().WithError(err).Message("Failed to get URL.")
		ginutil.WriteInvalidParamError(c.Context, err, "URL", fmt.Sprintf("Unable to parse URL %q", c.BaseURL))
		return Repository{}, false
	}

	log.Debug().WithStringer("url", urlPath).Message("Get repositories URL.")

	repositories := repositoryResponse{}
	err = requests.GetUnmarshalJSON(&repositories, c.UserName, c.Token, urlPath)
	if err != nil {
		log.Error().WithError(err).Message("Failed to get project repository.")
		ginutil.WriteProviderResponseError(c.Context, err,
			fmt.Sprintf(
				"Invalid response getting repositories from project %q in group %q. ",
				project.Name, groupName)+
				"Could be caused by invalid JSON data structure. "+
				"Might be the result of an incompatible version of Azure DevOps.")
		return Repository{}, false
	}

	if repositories.Count != 1 {
		log.Error().WithInt("repoCount", repositories.Count).Message("One repository is required.")
		err = errors.New("one repository is required")
		ginutil.WriteAPIClientReadError(c.Context, err,
			fmt.Sprintf("There were %d repositories, we need it to be 1.",
				repositories.Count))
		return Repository{}, false
	}

	freshProjectID := repositories.Value[0].Project.ID
	if freshProjectID != project.ID {
		log.Error().
			WithString("got", freshProjectID).
			WithString("want", project.ID).
			Message("Repository is not connected with project.")
		err = errors.New("repository is not connected with project")
		ginutil.WriteAPIClientReadError(c.Context, err,
			fmt.Sprintf("Repository ID (%s) and project ID (%s) mismatch.",
				freshProjectID,
				project.ID))
		return Repository{}, false
	}

	return repositories.Value[0], true
}

// GetFileWritesProblem attempts to get a file from the specified project using
// BasicAuth.
func (c *Client) GetFileWritesProblem(groupName, projectName, filePath string) (string, bool) {
	urlPath, err := c.newGetFile(groupName, projectName, filePath)
	if err != nil {
		log.Error().WithError(err).Message("Failed to get URL.")
		ginutil.WriteInvalidParamError(c.Context, err, "url", fmt.Sprintf("Unable to parse URL %q.", c.BaseURL))
		return "", false
	}

	log.Debug().WithStringer("url", urlPath).Message("Get file URL.")

	fileContents, err := requests.GetAsString(c.UserName, c.Token, urlPath)
	if err != nil {
		log.Error().
			WithError(err).
			WithStringf("project", "%s/%s", groupName, projectName).
			WithString("file", filePath).
			Message("Failed to fetch file from project.")
		ginutil.WriteFetchBuildDefinitionError(c.Context, err,
			fmt.Sprintf("Unable to fetch file from project %q.", projectName))
		return "", false
	}

	return fileContents, true
}

// GetProjectBranchesWritesProblem invokes a GET request to the remote provider,
// fetching the branches for the specified project.
func (c *Client) GetProjectBranchesWritesProblem(groupName, projectName, refsFilter string) ([]Branch, bool) {
	urlPath, err := c.newGetGitRefs(groupName, projectName, refsFilter)
	if err != nil {
		ginutil.WriteInvalidParamError(c.Context, err, "URL", fmt.Sprintf("Unable to parse URL %q", c.BaseURL))
		return []Branch{}, false
	}

	log.Debug().WithStringer("url", urlPath).Message("Get branches URL.")

	projectRefs := struct {
		Value []ref `json:"value"`
		Count int   `json:"count"`
	}{}

	err = requests.GetUnmarshalJSON(&projectRefs, c.UserName, c.Token, urlPath)
	if err != nil {
		ginutil.WriteProviderResponseError(c.Context, err,
			fmt.Sprintf(
				"Invalid response getting branches for project %q in group %q, using refs filter %q. ",
				projectName, groupName, refsFilter)+
				"Could be caused by invalid JSON data structure. "+
				"Might be the result of an incompatible version of Azure DevOps.")
		return []Branch{}, false
	}

	var projectBranches []Branch
	refsFilter = fmt.Sprintf("refs/%s", refsFilter)
	for _, ref := range projectRefs.Value {
		name := strings.TrimPrefix(ref.Name, refsFilter)
		projectBranches = append(projectBranches, Branch{
			Name: name,
			Ref:  ref.Name,
		})
	}

	return projectBranches, true
}

func (c *Client) newGetRepositories(groupName, projectName string) (*url.URL, error) {
	urlPath := *c.BaseURLParsed
	urlPath.Path = fmt.Sprintf("%s/%s/_apis/repositories", groupName, projectName)

	q := url.Values{}
	q.Add("api-version", "5.0")
	urlPath.RawQuery = q.Encode()

	return &urlPath, nil
}

func (c *Client) newGetFile(groupName, projectName, filePath string) (*url.URL, error) {
	urlPath := *c.BaseURLParsed
	urlPath.Path = fmt.Sprintf("%s/%s/_apis/repositories/%s/items",
		groupName, projectName, projectName)

	q := url.Values{}
	q.Add("scopePath", fmt.Sprintf("/%s", filePath))
	urlPath.RawQuery = q.Encode()

	return &urlPath, nil
}

func (c *Client) newGetProject(groupName, projectName string) (*url.URL, error) {
	urlPath := *c.BaseURLParsed
	urlPath.Path = fmt.Sprintf("%s/_apis/projects/%s", groupName, projectName)

	q := url.Values{}
	q.Add("api-version", "5.0")
	urlPath.RawQuery = q.Encode()

	return &urlPath, nil
}

func (c *Client) newGetProjects(groupName string) (*url.URL, error) {
	urlPath := *c.BaseURLParsed
	urlPath.Path = fmt.Sprintf("%s/_apis/projects", groupName)

	q := url.Values{}
	q.Add("api-version", "5.0")
	urlPath.RawQuery = q.Encode()

	return &urlPath, nil
}

func (c *Client) newGetGitRefs(groupName, projectName, refsFilter string) (*url.URL, error) {
	urlPath := *c.BaseURLParsed
	urlPath.Path = fmt.Sprintf("%s/%s/_apis/repositories/%s/refs",
		groupName, projectName, projectName)

	q := url.Values{}
	q.Add("api-version", "5.0")
	q.Add("filter", refsFilter)
	urlPath.RawQuery = q.Encode()

	return &urlPath, nil
}
