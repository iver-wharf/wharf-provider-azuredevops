package azure

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/iver-wharf/wharf-api-client-go/pkg/wharfapi"
	"github.com/iver-wharf/wharf-core/pkg/ginutil"
)

const (
	apiProviderName         = "azuredevops"
	buildDefinitionFileName = ".wharf-ci.yml"
	apiProjects             = "_apis/projects"
)

// ImportData holds the data received from the user.
type ImportData struct {
	// used in refresh only
	TokenID   uint   `json:"tokenId" example:"0"`
	Token     string `json:"token" example:"sample token"`
	UserName  string `json:"user" example:"sample user name"`
	URL       string `json:"url" example:"https://gitlab.local"`
	UploadURL string `json:"uploadUrl" example:""`
	// used in refresh only
	ProviderID uint `json:"providerId" example:"0"`
	// azuredevops, gitlab or github
	Provider string `json:"provider" example:"gitlab"`
	// used in refresh only
	ProjectID   uint   `json:"projectId" example:"0"`
	ProjectName string `json:"project" example:"sample project name"`
	GroupName   string `json:"group" example:"default"`
}

type azureCreator struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	URL         string `json:"url"`
	UniqueName  string `json:"uniqueName"`
	ImageURL    string `json:"imageUrl"`
	Descriptor  string `json:"descriptor"`
}

type azureRef struct {
	ObjectID string       `json:"objectId"`
	Name     string       `json:"name"`
	Creator  azureCreator `json:"creator"`
	URL      string       `json:"url"`
}

type azureProject struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	State       string `json:"state"`
	Revision    int64  `json:"revision"`
	Visibility  string `json:"visibility"`
}

// azureProjectResponse is the response received from the remote provider
// when getting project(s).
type azureProjectResponse struct {
	Count int            `json:"count"`
	Value []azureProject `json:"value"`
}

type azureRepository struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	URL           string       `json:"url"`
	Project       azureProject `json:"project"`
	DefaultBranch string       `json:"defaultBranch"`
	Size          int64        `json:"size"`
	RemoteURL     string       `json:"remoteUrl"`
	SSHURL        string       `json:"sshUrl"`
}

type azureRepositoryResponse struct {
	Count int               `json:"count"`
	Value []azureRepository `json:"value"`
}

type azureBranch struct {
	Name          string
	Ref           string
	DefaultBranch bool
}

type azureImporter struct {
	context     *gin.Context
	azureClient *azureClient
	wharfClient *wharfapi.Client
	data        ImportData
	// retrieved from database
	token wharfapi.Token
	// retrieved from database
	provider wharfapi.Provider
}

func NewImporterWritesProblem(c *gin.Context, wharfClient *wharfapi.Client) (*azureImporter, bool) {
	i := ImportData{}
	err := c.ShouldBindJSON(&i)
	if err != nil {
		ginutil.WriteInvalidBindError(c, err,
			"One or more parameters failed to parse when reading the request body for import details.")
		return nil, false
	}

	fmt.Println("from json: ", i)

	if i.GroupName == "" {
		fmt.Println("Unable to get due to empty group.")
		err := errors.New("missing required property: group")
		ginutil.WriteInvalidParamError(c, err, "group",
			"Unable to import due to empty group.")
		return nil, false
	}

	importer := azureImporter{
		context:     c,
		wharfClient: wharfClient,
		data:        i}

	var ok bool
	importer.token, ok = importer.getOrPostTokenWritesProblem()
	if !ok {
		fmt.Println("Unable to get or create token.")
		return nil, false
	}
	fmt.Println("Token from db: ", i.Token)

	importer.provider, ok = importer.getOrPostProviderWritesProblem()
	if !ok {
		return nil, false
	}
	fmt.Println("Provider from db: ", i.Provider)

	importer.azureClient = &azureClient{
		context:   c,
		Token:     importer.token,
		Provider:  importer.provider,
		UserName:  i.UserName,
		URL:       i.URL,
		UploadURL: i.UploadURL}

	return &importer, true
}

// PutProjectWritesProblem attempts to insert a project into the Wharf database.
func (i *azureImporter) PutProjectWritesProblem(project azureProject) (wharfapi.Project, bool) {
	buildDefinitionStr, ok := i.azureClient.getBuildDefinitionWritesProblem(i.data.GroupName, project.Name, buildDefinitionFileName)
	if !ok {
		return wharfapi.Project{}, false
	}

	gitURL, err := i.azureClient.getGitURL(i.provider.URL, i.data.GroupName, project.Name)
	if err != nil {
		fmt.Println("Unable to construct git url ", err)
		ginutil.WriteComposingProviderDataError(i.context, err,
			fmt.Sprintf("Unable to construct git url for project '%s' in group '%s'", project.Name, i.data.GroupName))
		return wharfapi.Project{}, false
	}

	projectInDB, err := i.wharfClient.PutProject(wharfapi.Project{
		Name:            project.Name,
		TokenID:         i.token.TokenID,
		GroupName:       i.data.GroupName,
		BuildDefinition: buildDefinitionStr,
		Description:     project.Description,
		ProviderID:      i.provider.ProviderID,
		GitURL:          gitURL})

	if err != nil {
		fmt.Println("Unable to put project: ", err)
		ginutil.WriteAPIClientWriteError(i.context, err,
			fmt.Sprintf("Unable to import project '%s' from group '%s' at url '%s'.",
				i.data.ProjectName, i.data.GroupName, gitURL))
		return wharfapi.Project{}, false
	}

	return projectInDB, true
}

// PostBranchesWritesProblem attempts to insert a project's branches into the
// Wharf database.
func (i *azureImporter) PostBranchesWritesProblem(project azureProject, projectInDB wharfapi.Project) bool {
	repositories, ok := i.azureClient.getRepositoriesWritesProblem(i.data.GroupName, project)
	if !ok {
		return false
	}

	projectBranches, ok := i.azureClient.getProjectBranchesWritesProblem(i.data.GroupName, project.Name, "heads/")
	if !ok {
		return false
	}

	for _, branch := range projectBranches {
		_, err := i.wharfClient.PutBranch(wharfapi.Branch{
			Name:      branch.Name,
			ProjectID: projectInDB.ProjectID,
			Default:   branch.Ref == repositories.Value[0].DefaultBranch,
			TokenID:   i.token.TokenID,
		})
		if err != nil {
			fmt.Println("Unable to post branch: ", err)
			ginutil.WriteAPIClientWriteError(i.context, err, fmt.Sprintf("Unable to import branch %q", branch.Name))
			return false
		}
	}

	return true
}

func (i *azureImporter) getTokenByIDWritesProblem(tokenID uint) (wharfapi.Token, bool) {
	token, err := i.wharfClient.GetTokenById(tokenID)
	if err != nil || token.TokenID == 0 {
		fmt.Printf("Unable to get token. %+v", err)
		ginutil.WriteAPIClientReadError(i.context, err,
			fmt.Sprintf("Unable to get token by ID %d.", tokenID))
		return wharfapi.Token{}, false
	}

	return token, true
}

func (i *azureImporter) getOrPostTokenWritesProblem() (wharfapi.Token, bool) {
	if i.data.UserName == "" && i.data.TokenID == 0 {
		err := errors.New("both token and user were omitted")
		ginutil.WriteInvalidParamError(i.context, err, "user",
			"Unable to import when both user and token are omitted.")
		return wharfapi.Token{}, false
	}

	var token wharfapi.Token
	if i.token.TokenID != 0 {
		var ok bool
		token, ok = i.getTokenByIDWritesProblem(i.token.TokenID)
		if !ok {
			return wharfapi.Token{}, false
		}
	} else {
		var err error
		token, err = i.wharfClient.GetToken(i.token.Token, i.data.UserName)
		if err != nil || token.TokenID == 0 {
			token, err = i.wharfClient.PostToken(wharfapi.Token{
				Token:    i.token.Token,
				UserName: i.data.UserName})
			if err != nil {
				fmt.Println("Unable to post token: ", err)
				ginutil.WriteAPIClientWriteError(i.context, err, "Unable to get existing token or create new token.")
				return wharfapi.Token{}, false
			}
		}
	}

	return token, true
}

func (i *azureImporter) getOrPostProviderWritesProblem() (wharfapi.Provider, bool) {
	var provider wharfapi.Provider
	if i.data.ProviderID != 0 {
		provider, err := i.wharfClient.GetProviderById(i.provider.ProviderID)
		if err != nil || provider.ProviderID == 0 {
			fmt.Printf("Unable to get provider. %+v", err)
			ginutil.WriteAPIClientReadError(i.context, err,
				fmt.Sprintf("Unable to get provider by ID %d", i.data.ProviderID))
			return wharfapi.Provider{}, false
		}
	} else {
		var err error
		provider, err = i.wharfClient.GetProvider(
			apiProviderName,
			i.data.URL,
			i.data.UploadURL,
			i.token.TokenID)
		if err != nil || provider.ProviderID == 0 {
			provider, err = i.wharfClient.PostProvider(
				wharfapi.Provider{
					Name:    apiProviderName,
					URL:     i.data.URL,
					TokenID: i.token.TokenID})
			if err != nil {
				fmt.Println("Unable to post provider: ", err)
				ginutil.WriteAPIClientWriteError(i.context, err,
					fmt.Sprintf("Unable to get or create provider from '%s'.", i.data.URL))
				return wharfapi.Provider{}, false
			}
		}
	}

	return provider, true
}

func (i *azureImporter) GetProjectsWritesProblem() (azureProjectResponse, bool) {
	if i.data.ProjectName != "" {
		return i.azureClient.GetProjectWritesProblem(i.data.GroupName, i.data.ProjectName)
	}

	return i.azureClient.GetProjectsWritesProblem(i.data.GroupName)
}
