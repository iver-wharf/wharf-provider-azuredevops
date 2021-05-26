package main

import (
	"fmt"
	"os"
	"strconv"

	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/iver-wharf/wharf-api-client-go/pkg/wharfapi"
	_ "github.com/iver-wharf/wharf-provider-azuredevops/docs"
)

const apiRepositories string = "_apis/git/repositories"
const apiProjects string = "_apis/projects"
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
// @Success 200 "OK"
// @Failure 400 "Bad request"
// @Failure 401 "Unauthorized or missing jwt token"
// @Router /azuredevops [post]
func runAzureDevOpsHandler(c *gin.Context) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	client := wharfapi.Client{
		ApiUrl:     os.Getenv("WHARF_API_URL"),
		AuthHeader: c.GetHeader("Authorization"),
	}

	i := importBody{}
	err := c.BindJSON(&i)
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusBadRequest, err)
		return
	}

	fmt.Println("from json: ", i)

	if i.Group == "" {
		fmt.Println("Unable to get due to empty group.")
		c.JSON(http.StatusBadRequest, "Unable to get due to empty group.")
		return
	}

	var token wharfapi.Token
	if i.TokenID != 0 {
		token, err = client.GetTokenById(i.TokenID)
		if err != nil || token.TokenID == 0 {
			fmt.Printf("Unable to get token. %+v", err)
			c.JSON(http.StatusBadRequest, fmt.Sprintf("Unable to get token. %+v", err))
			return
		}
		i.User = token.UserName
		i.Token = token.Token
	} else if i.User == "" {
		fmt.Println("Unable to get due to empty user.")
		c.JSON(http.StatusBadRequest, "Unable to get due to empty user.")
		return
	} else {
		token, err = client.GetToken(i.Token, i.User)
		if err != nil || token.TokenID == 0 {
			token, err = client.PostToken(wharfapi.Token{Token: i.Token, UserName: i.User})
			if err != nil {
				fmt.Println("Unable to put token: ", err)
				c.JSON(http.StatusBadRequest, fmt.Sprintf("Error: %+v", err))
				return
			}
		}
	}
	fmt.Println("Token from db: ", token)

	var provider wharfapi.Provider
	if i.ProviderID != 0 {
		provider, err = client.GetProviderById(i.ProviderID)
		if err != nil || provider.ProviderID == 0 {
			fmt.Printf("Unable to get provider. %+v", err)
			c.JSON(http.StatusBadRequest, fmt.Sprintf("Unable to get provider. %+v", err))
			return
		}
		i.URL = provider.URL
	} else {
		provider, err = client.GetProvider("azuredevops", i.URL, i.UploadURL, token.TokenID)
		if err != nil || provider.ProviderID == 0 {
			provider, err = client.PostProvider(wharfapi.Provider{Name: "azuredevops", URL: i.URL, TokenID: token.TokenID})
			if err != nil {
				fmt.Println("Unable to put provider: ", err)
				c.JSON(http.StatusBadRequest, fmt.Sprintf("Error: %+v", err))
				return
			}
		}
	}
	fmt.Println("Provider from db: ", provider)

	url, err := buildURL(i.URL, i.Group, i.Project)
	if err != nil {
		fmt.Println("Unable to build url: ", err)
		c.JSON(http.StatusBadRequest, fmt.Sprintf("Error: %+v", err))
		return
	}

	bodyBytes, err := getBodyFromRequest(i.User, i.Token, url)
	if err != nil {
		c.JSON(http.StatusBadRequest, fmt.Sprintf("Error: %+v", err))
		return
	}

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	projects := struct {
		Value []azureDevOpsProject `json:"value"`
		Count int                  `json:"count"`
	}{
		Count: 1,
		Value: make([]azureDevOpsProject, 1)}
	if i.Project != "" {
		err = json.Unmarshal(bodyBytes, &projects.Value[0])
	} else {
		err = json.Unmarshal(bodyBytes, &projects)
	}
	if err != nil {
		fmt.Println("Unable to unmarshal projects: ", err)
		c.JSON(http.StatusBadRequest, fmt.Sprintf("Error: %+v", err))
		return
	}

	for _, project := range projects.Value {
		buildDefinitionStr, err := getAzureDevOpsBuildDefinition(i, project.Name)
		if err != nil {
			fmt.Println("Unable to get build definition: ", err)
			c.JSON(http.StatusBadRequest, fmt.Sprintf("Error: %+v", err))
			return
		}

		gitURL, err := getGitURL(provider, i.Group, project)
		if err != nil {
			fmt.Println("Unable to construct git url ", err)
			c.JSON(http.StatusBadRequest, fmt.Sprintf("Error: %+v", err))
			return
		}

		projectInDb, err := client.PutProject(
			wharfapi.Project{
				Name:            project.Name,
				TokenID:         token.TokenID,
				GroupName:       i.Group,
				BuildDefinition: buildDefinitionStr,
				Description:     project.Description,
				ProviderID:      provider.ProviderID,
				GitURL:          gitURL})

		if err != nil {
			fmt.Println("Unable to put project: ", err)
			c.JSON(http.StatusBadRequest, fmt.Sprintf("Error: %+v", err))
			return
		}

		repositoryBody, err := getRepositories(i, project.Name)
		if err != nil {
			fmt.Println("Unable to get project repository: ", err)
			continue
		}

		repositories := struct {
			Value []azureDevOpsRepository `json:"value"`
			Count int                     `json:"count"`
		}{}
		err = json.Unmarshal(repositoryBody, &repositories)
		if err != nil {
			fmt.Println("Unable to unmarshal repository: ", err)
			continue
		}

		if repositories.Count == 0 || repositories.Count > 1 {
			fmt.Println("One repository is required.")
			continue
		}

		if repositories.Value[0].Project.ID != project.ID {
			fmt.Println("Repository is not connected with project.")
			continue
		}

		projectBranches, err := getProjectBranches(i, project.Name)
		if err != nil {
			fmt.Println("Unable to get project branches: ", err)
			continue
		}

		for _, branch := range projectBranches {
			_, err := client.PutBranch(
				wharfapi.Branch{
					Name:      branch.Name,
					ProjectID: projectInDb.ProjectID,
					Default:   branch.Ref == repositories.Value[0].DefaultBranch,
					TokenID:   token.TokenID,
				})
			if err != nil {
				fmt.Println("Unable to put branch: ", err)
				c.JSON(http.StatusBadRequest, fmt.Sprintf("Error: %+v", err))
				break
			}
		}
	}

	c.JSON(http.StatusOK, "OK")
}

// prCreatedTriggerHandler godoc
// @Summary Triggers prcreated action on wharf-client
// @Accept json
// @Produce json
// @Param projectid path int true "wharf project ID"
// @Param azureDevOpsPR body azureDevOpsPR _ "AzureDevOps PR "
// @Param environment query string true "wharf build environment"
// @Success 200 "OK"
// @Failure 400 "Bad request"
// @Failure 401 "Unauthorized or missing jwt token"
// @Router /azuredevops/triggers/{projectid}/pr/created [post]
func prCreatedTriggerHandler(c *gin.Context) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	t := azureDevOpsPR{}
	if err := c.BindJSON(&t); err != nil {
		c.Error(err)
		c.JSON(http.StatusBadRequest, err)
		return
	}

	if t.EventType != "git.pullrequest.created" {
		c.JSON(
			http.StatusBadRequest,
			fmt.Sprintf("Expected git.pullrequest.created trigger, got: %s instead.", t.EventType))
		return
	}

	projectID, err := strconv.ParseUint(c.Param("projectid"), 10, 32)
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusBadRequest, fmt.Sprintf("Could not get projectid from query string %s", err))
		return
	}

	environment := c.Query("environment")

	client := wharfapi.Client{
		ApiUrl:     os.Getenv("WHARF_API_URL"),
		AuthHeader: c.GetHeader("Authorization"),
	}

	var resp wharfapi.ProjectRunResponse
	resp, err2 := client.PostProjectRun(
		wharfapi.ProjectRun{
			ProjectID:   uint(projectID),
			Stage:       "prcreatedd",
			Branch:      strings.TrimPrefix(t.Resource.SourceRefName, "refs/heads/"),
			Environment: environment,
		},
	)

	if err2 != nil {
		fmt.Println("Unable to send trigger to wharf-client: ", err2)
		c.JSON(http.StatusBadRequest, fmt.Sprintf("Unable to send trigger to wharf-client, error: %v", err2))
		return
	}

	c.JSON(http.StatusOK, resp)
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

func getBodyFromRequest(user string, token string, url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		fmt.Println("Unable to get: ", err)
		return []byte{}, err
	}

	req.SetBasicAuth(user, token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Unable to do request: ", err)
		return []byte{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Unable to get. Status code: ", resp.StatusCode)
		return []byte{}, err
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return []byte{}, err
	}

	return bodyBytes, nil
}

func getProjectBranches(i importBody, project string) ([]azureDevOpsBranch, error) {
	urlPath, err := url.Parse(i.URL)
	if err != nil {
		fmt.Println("Unable to get url: ", err)
		return []azureDevOpsBranch{}, err
	}

	urlPath.Path = fmt.Sprintf("%v/%v/%v/%v/%v", i.Group, project, apiRepositories, project, refsPath)
	q := url.Values{}
	q.Add("api-version", "5.0")
	q.Add("filter", "heads/")
	urlPath.RawQuery = q.Encode()

	fmt.Println(urlPath.String())

	body, err := getBodyFromRequest(i.User, i.Token, urlPath.String())

	projectRefs := struct {
		Value []azureDevOpsRef `json:"value"`
		Count int              `json:"count"`
	}{}

	err = json.Unmarshal(body, &projectRefs)
	if err != nil {
		fmt.Println("Unable to unmarshal refs: ", err)
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

func getRepositories(i importBody, project string) ([]byte, error) {
	urlPath, err := url.Parse(i.URL)
	if err != nil {
		fmt.Println("Unable to get url: ", err)
		return []byte{}, err
	}

	urlPath.Path = fmt.Sprintf("%v/%v/%v", i.Group, project, apiRepositories)
	data := url.Values{}
	data.Set("api-version", "5.0")
	urlPath.RawQuery = data.Encode()
	fmt.Println(urlPath.String())

	return getBodyFromRequest(i.User, i.Token, urlPath.String())
}

func getAzureDevOpsBuildDefinition(i importBody, project string) (string, error) {
	urlPath, err := url.Parse(i.URL)
	if err != nil {
		fmt.Println("Unable to get url: ", err)
		return "", err
	}

	urlPath.Path = fmt.Sprintf("%v/%v/%v/%v/%v", i.Group, project, apiRepositories, project, itemsPath)
	data := url.Values{}
	data.Set("scopePath", fmt.Sprintf("/%v", buildDefinitionFileName))
	urlPath.RawQuery = data.Encode()

	fmt.Println(urlPath.String())

	bodyBytes, err := getBodyFromRequest(i.User, i.Token, urlPath.String())
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	bodyString := string(bodyBytes)
	return bodyString, nil
}

func buildURL(urlStr string, group string, project string) (string, error) {
	urlPath, err := url.Parse(urlStr)
	if err != nil {
		fmt.Println("Unable to get url: ", err)
		return "", err
	}

	data := url.Values{}
	data.Set("api-version", "5.0")

	if project != "" {
		urlPath.Path = fmt.Sprintf("%v/%v/%v", group, apiProjects, project)
	} else {
		urlPath.Path = fmt.Sprintf("%v/%v", group, apiProjects)
	}

	urlPath.RawQuery = data.Encode()
	fmt.Println(urlPath.String())

	return urlPath.String(), nil
}
