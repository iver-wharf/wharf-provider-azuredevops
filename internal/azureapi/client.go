package azureapi

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
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
// matching the provided organization and project name.
func (c *Client) GetProjectWritesProblem(orgName, projectNameOrID string) (Project, bool) {
	getProjectURL, err := c.newGetProject(orgName, projectNameOrID)

	if err != nil {
		errorDetail := fmt.Sprintf("Unable to build url %q for '%s/_apis/projects/%s'",
			c.BaseURL, orgName, projectNameOrID)

		ginutil.WriteInvalidParamError(c.Context, err, "url", errorDetail)
		return Project{}, false
	}

	var project Project
	err = requests.GetUnmarshalJSON(&project, c.UserName, c.Token, getProjectURL)

	if err != nil {
		ginutil.WriteProviderResponseError(c.Context, err,
			fmt.Sprintf("Invalid response when getting project %q from organization %q. ", projectNameOrID, orgName)+
				"Could be caused by invalid JSON data structure. "+
				"Might be the result of an incompatible version of Azure DevOps.")
		return Project{}, false
	}

	return project, true
}

// GetProjectsWritesProblem attempts to get all projects from the specified URL
// that are part of the provided organization.
func (c *Client) GetProjectsWritesProblem(orgName string) ([]Project, bool) {
	getProjectsURL, err := c.newGetProjects(orgName)

	if err != nil {
		errorDetail := fmt.Sprintf("Unable to build url %q for '%s/_apis/projects'",
			c.BaseURL, orgName)

		ginutil.WriteInvalidParamError(c.Context, err, "URL", errorDetail)
		return []Project{}, false
	}

	var projects struct {
		Count int       `json:"count"`
		Value []Project `json:"value"`
	}

	err = requests.GetUnmarshalJSON(&projects, c.UserName, c.Token, getProjectsURL)
	if err != nil {
		ginutil.WriteProviderResponseError(c.Context, err,
			fmt.Sprintf("Invalid response getting projects from organization %q. ", orgName)+
				"Could be caused by invalid JSON data structure. "+
				"Might be the result of an incompatible version of Azure DevOps.")
		return []Project{}, false
	}

	return projects.Value, true
}

// GetRepositoryWritesProblem attempts to get a single repository for the
// specified project using BasicAuth.
func (c *Client) GetRepositoryWritesProblem(orgName, projectNameOrID, repoNameOrID string) (Repository, bool) {
	urlPath, err := c.newGetRepository(orgName, projectNameOrID, repoNameOrID)
	if err != nil {
		log.Error().WithError(err).Message("Failed to get URL.")
		ginutil.WriteInvalidParamError(c.Context, err, "URL", fmt.Sprintf("Unable to parse URL %q", c.BaseURL))
		return Repository{}, false
	}

	log.Debug().WithStringer("url", urlPath).Message("Get repository URL.")

	var repository Repository
	err = requests.GetUnmarshalJSON(&repository, c.UserName, c.Token, urlPath)
	if err != nil {
		log.Error().WithError(err).Message("Failed to get project repository.")
		ginutil.WriteProviderResponseError(c.Context, err,
			fmt.Sprintf(
				"Invalid response getting repository from repo %q from project %q in organization %q. ",
				repoNameOrID, projectNameOrID, orgName)+
				"Could be caused by invalid JSON data structure. "+
				"Might be the result of an incompatible version of Azure DevOps.")
		return Repository{}, false
	}

	return repository, true
}

// GetRepositoriesWritesProblem attempts to get all repositories for the
// specified project using BasicAuth.
func (c *Client) GetRepositoriesWritesProblem(orgName, projectNameOrID string) ([]Repository, bool) {
	urlPath, err := c.newGetRepositories(orgName, projectNameOrID)
	if err != nil {
		log.Error().WithError(err).Message("Failed to get URL.")
		ginutil.WriteInvalidParamError(c.Context, err, "URL", fmt.Sprintf("Unable to parse URL %q", c.BaseURL))
		return []Repository{}, false
	}

	log.Debug().WithStringer("url", urlPath).Message("Get repositories URL.")

	var repositories struct {
		Count int          `json:"count"`
		Value []Repository `json:"value"`
	}
	err = requests.GetUnmarshalJSON(&repositories, c.UserName, c.Token, urlPath)
	if err != nil {
		log.Error().WithError(err).Message("Failed to get project repository.")
		ginutil.WriteProviderResponseError(c.Context, err,
			fmt.Sprintf(
				"Invalid response getting repositories from project %q in organization %q. ",
				projectNameOrID, orgName)+
				"Could be caused by invalid JSON data structure. "+
				"Might be the result of an incompatible version of Azure DevOps.")
		return []Repository{}, false
	}

	return repositories.Value, true
}

// GetFileWritesProblem attempts to get a file from the specified project using
// BasicAuth.
func (c *Client) GetFileWritesProblem(orgName, projectNameOrID, repoNameOrID, filePath string) (string, bool) {
	urlPath, err := c.newGetFile(orgName, projectNameOrID, repoNameOrID, filePath)
	if err != nil {
		log.Error().WithError(err).Message("Failed to get URL.")
		ginutil.WriteInvalidParamError(c.Context, err, "url", fmt.Sprintf("Unable to parse URL %q.", c.BaseURL))
		return "", false
	}

	log.Debug().WithStringer("url", urlPath).Message("Get file URL.")

	fileContents, err := requests.GetAsString(c.UserName, c.Token, urlPath)
	var non2xxErr requests.Non2xxStatusError
	if errors.As(err, &non2xxErr) && non2xxErr.StatusCode == http.StatusNotFound {
		log.Debug().
			WithError(err).
			WithString("org", orgName).
			WithString("project", projectNameOrID).
			WithString("repo", repoNameOrID).
			WithString("file", filePath).
			Message("File not found in project.")
		return "", true
	} else if err != nil {
		log.Error().
			WithError(err).
			WithString("org", orgName).
			WithString("project", projectNameOrID).
			WithString("repo", repoNameOrID).
			WithString("file", filePath).
			Message("Failed to fetch file from project.")
		ginutil.WriteFetchBuildDefinitionError(c.Context, err,
			fmt.Sprintf("Unable to fetch file from project %q.", projectNameOrID))
		return "", false
	}

	return fileContents, true
}

// GetRepositoryBranchesWritesProblem invokes a GET request to the remote
// provider, fetching the branches for the specified repository.
func (c *Client) GetRepositoryBranchesWritesProblem(orgName, projectNameOrID, repoNameOrID string) ([]Branch, bool) {
	const refBranchesFilter = "heads/"
	const refBranchesPrefix = "refs/" + refBranchesFilter

	urlPath, err := c.newGetGitRefs(orgName, projectNameOrID, repoNameOrID, refBranchesFilter)
	if err != nil {
		ginutil.WriteInvalidParamError(c.Context, err, "URL", fmt.Sprintf("Unable to parse URL %q", c.BaseURL))
		return []Branch{}, false
	}

	log.Debug().WithStringer("url", urlPath).Message("Get branches URL.")

	var projectRefs struct {
		Value []struct {
			ObjectID string  `json:"objectId"`
			Name     string  `json:"name"`
			Creator  creator `json:"creator"`
			URL      string  `json:"url"`
		} `json:"value"`
		Count int `json:"count"`
	}
	err = requests.GetUnmarshalJSON(&projectRefs, c.UserName, c.Token, urlPath)
	if err != nil {
		ginutil.WriteProviderResponseError(c.Context, err,
			fmt.Sprintf(
				"Invalid response getting branches for project %q in organization %q, using refs filter %q. ",
				projectNameOrID, orgName, refBranchesFilter)+
				"Could be caused by invalid JSON data structure. "+
				"Might be the result of an incompatible version of Azure DevOps.")
		return []Branch{}, false
	}

	var projectBranches []Branch
	for _, ref := range projectRefs.Value {
		name := strings.TrimPrefix(ref.Name, refBranchesPrefix)
		projectBranches = append(projectBranches, Branch{
			Name: name,
			Ref:  ref.Name,
		})
	}

	return projectBranches, true
}

func (c *Client) newGetRepository(orgName, projectNameOrID, repoNameOrID string) (*url.URL, error) {
	urlPath := c.newUrlWithPath("%s/%s/_apis/git/repositories/%s",
		orgName, projectNameOrID, repoNameOrID)

	q := url.Values{}
	q.Add("api-version", "5.0")
	urlPath.RawQuery = q.Encode()

	return &urlPath, nil
}

func (c *Client) newGetRepositories(orgName, projectNameOrID string) (*url.URL, error) {
	urlPath := c.newUrlWithPath("%s/%s/_apis/git/repositories", orgName, projectNameOrID)

	q := url.Values{}
	q.Add("api-version", "5.0")
	urlPath.RawQuery = q.Encode()

	return &urlPath, nil
}

func (c *Client) newGetFile(orgName, projectNameOrID, repoNameOrID, filePath string) (*url.URL, error) {
	urlPath := c.newUrlWithPath("%s/%s/_apis/git/repositories/%s/items",
		orgName, projectNameOrID, repoNameOrID)

	q := url.Values{}
	q.Add("scopePath", fmt.Sprintf("/%s", filePath))
	urlPath.RawQuery = q.Encode()

	return &urlPath, nil
}

func (c *Client) newGetProject(orgName, projectNameOrID string) (*url.URL, error) {
	urlPath := c.newUrlWithPath("%s/_apis/projects/%s", orgName, projectNameOrID)

	q := url.Values{}
	q.Add("api-version", "5.0")
	urlPath.RawQuery = q.Encode()

	return &urlPath, nil
}

func (c *Client) newGetProjects(orgName string) (*url.URL, error) {
	urlPath := c.newUrlWithPath("%s/_apis/projects", orgName)

	q := url.Values{}
	q.Add("api-version", "5.0")
	urlPath.RawQuery = q.Encode()

	return &urlPath, nil
}

func (c *Client) newGetGitRefs(orgName, projectNameOrID, repoNameOrID, refsFilter string) (*url.URL, error) {
	urlPath := c.newUrlWithPath("%s/%s/_apis/git/repositories/%s/refs",
		orgName, projectNameOrID, repoNameOrID)

	q := url.Values{}
	q.Add("api-version", "5.0")
	q.Add("filter", refsFilter)
	urlPath.RawQuery = q.Encode()

	return &urlPath, nil
}

func (c *Client) newUrlWithPath(format string, args ...any) url.URL {
	u := *c.BaseURLParsed
	u.Path = path.Join(u.Path, fmt.Sprintf(format, args...))
	return u
}
