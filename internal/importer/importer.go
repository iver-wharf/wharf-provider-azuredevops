package importer

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/iver-wharf/wharf-api-client-go/pkg/wharfapi"
	"github.com/iver-wharf/wharf-core/pkg/ginutil"
	"github.com/iver-wharf/wharf-provider-azuredevops/internal/azureapi"
)

const (
	apiProviderName         = "azuredevops"
	buildDefinitionFileName = ".wharf-ci.yml"
)

// Importer is an interface for importing project data from a remote provider to
// the Wharf API.
//
// All of the functions will write a problem to the provided gin.Context when an
// error occurs.
type Importer interface {
	// Init initializes the importer, verifying the token and provider data.
	//
	// Writes a problem to the provided gin.Context on failure.
	Init(token wharfapi.Token, provider wharfapi.Provider, c *gin.Context, client wharfapi.Client) bool
	// ImportProjectInGroup imports a single project from a specific
	// group/organization.
	//
	// Writes a problem to the provided gin.Context on failure.
	ImportProjectInGroup(groupName, projectName string) bool
	// ImportAllProjectsInGroup imports all projects from a specific
	// group/organization.
	//
	// Writes a problem to the provided gin.Context on failure.
	ImportAllProjectsInGroup(groupName string) bool
}

type azureImporter struct {
	c     *gin.Context
	wharf *wharfapi.Client
	azure *azureapi.Client
	// retrieved from database
	token wharfapi.Token
	// retrieved from database
	provider wharfapi.Provider
}

func (i azureImporter) Init(token wharfapi.Token, provider wharfapi.Provider, c *gin.Context, client wharfapi.Client) bool {
	var ok bool
	i.token, ok = i.getOrPostTokenWritesProblem(token)
	if !ok {
		fmt.Println("Unable to get or create token.")
		return false
	}
	fmt.Println("Token from db: ", i.token)

	i.provider, ok = i.getOrPostProviderWritesProblem(provider)
	if !ok {
		return false
	}
	fmt.Println("Provider from db: ", i.provider)

	i.wharf = &client
	i.azure = &azureapi.Client{
		Context:  c,
		BaseURL:  i.provider.URL,
		UserName: i.token.UserName,
		Token:    i.token.Token,
	}

	return true
}

func (i azureImporter) ImportProjectInGroup(groupName, projectName string) bool {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	project, ok := i.azure.GetProjectWritesProblem(groupName, projectName)
	if !ok {
		return false
	}

	i.putProjectToWharfWithBranchesWritesProblem(groupName, project)

	return true
}

func (i azureImporter) ImportAllProjectsInGroup(groupName string) bool {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	projects, ok := i.azure.GetProjectsWritesProblem(groupName)
	if !ok {
		return false
	}

	for _, project := range projects {
		if ok := i.putProjectToWharfWithBranchesWritesProblem(groupName, project); !ok {
			return false
		}
	}

	return true
}

// NewAzureImporter creates a new azureImporter.
func NewAzureImporter(c *gin.Context, client *wharfapi.Client) Importer {
	return azureImporter{
		c:     c,
		wharf: client,
	}
}

func (i azureImporter) putProjectToWharfWithBranchesWritesProblem(groupName string, project azureapi.Project) bool {
	projectInDB, ok := i.putProjectToWharfWritesProblem(groupName, project)
	if !ok {
		fmt.Printf("Unable to import project %q", project.Name)
		return false
	}

	ok = i.postBranchesToWharfWritesProblem(groupName, project, projectInDB)
	if !ok {
		fmt.Printf("Unable to import branches from project %q", project.Name)
		return false
	}

	return true
}

func (i *azureImporter) putProjectToWharfWritesProblem(groupName string, project azureapi.Project) (wharfapi.Project, bool) {
	buildDefinitionStr, ok := i.azure.GetFileWritesProblem(groupName, project.Name, buildDefinitionFileName)
	if !ok {
		return wharfapi.Project{}, false
	}

	gitURL, err := i.constructGitURL(groupName, project.Name)
	if err != nil {
		fmt.Println("Unable to construct git url ", err)
		ginutil.WriteComposingProviderDataError(i.c, err,
			fmt.Sprintf("Unable to construct git url for project %q in group %q", project.Name, groupName))
		return wharfapi.Project{}, false
	}

	projectInDB, err := i.wharf.PutProject(wharfapi.Project{
		Name:            project.Name,
		TokenID:         i.token.TokenID,
		GroupName:       groupName,
		BuildDefinition: buildDefinitionStr,
		Description:     project.Description,
		ProviderID:      i.provider.ProviderID,
		GitURL:          gitURL})

	if err != nil {
		fmt.Println("Unable to put project: ", err)
		ginutil.WriteAPIClientWriteError(i.c, err,
			fmt.Sprintf("Unable to import project %q from group %q at url %q.",
				project.Name, groupName, gitURL))
		return wharfapi.Project{}, false
	}

	return projectInDB, true
}

func (i *azureImporter) postBranchesToWharfWritesProblem(groupName string, project azureapi.Project, projectInDB wharfapi.Project) bool {
	repository, ok := i.azure.GetRepositoryWritesProblem(groupName, project)
	if !ok {
		return false
	}

	projectBranches, ok := i.azure.GetProjectBranchesWritesProblem(groupName, project.Name, "heads/")
	if !ok {
		return false
	}

	for _, branch := range projectBranches {
		_, err := i.wharf.PutBranch(wharfapi.Branch{
			Name:      branch.Name,
			ProjectID: projectInDB.ProjectID,
			Default:   branch.Ref == repository.DefaultBranch,
			TokenID:   i.token.TokenID,
		})
		if err != nil {
			fmt.Println("Unable to post branch: ", err)
			ginutil.WriteAPIClientWriteError(i.c, err, fmt.Sprintf("Unable to import branch %q", branch.Name))
			return false
		}
	}

	return true
}

func (i *azureImporter) getTokenByIDWritesProblem(tokenID uint) (wharfapi.Token, bool) {
	token, err := i.wharf.GetTokenById(tokenID)
	if err != nil || token.TokenID == 0 {
		fmt.Printf("Unable to get token. %+v", err)
		ginutil.WriteAPIClientReadError(i.c, err,
			fmt.Sprintf("Unable to get token by ID %d.", tokenID))
		return wharfapi.Token{}, false
	}

	return token, true
}

func (i *azureImporter) getOrPostTokenWritesProblem(token wharfapi.Token) (wharfapi.Token, bool) {
	if token.UserName == "" && token.TokenID == 0 {
		err := errors.New("both token and user were omitted")
		ginutil.WriteInvalidParamError(i.c, err, "user",
			"Unable to import when both user and token are omitted.")
		return wharfapi.Token{}, false
	}

	if token.TokenID != 0 {
		var ok bool
		token, ok = i.getTokenByIDWritesProblem(token.TokenID)
		if !ok {
			return wharfapi.Token{}, false
		}
	} else {
		var err error
		token, err = i.wharf.GetToken(token.Token, token.UserName)
		if err != nil || token.TokenID == 0 {
			token, err = i.wharf.PostToken(wharfapi.Token{
				Token:    token.Token,
				UserName: token.UserName})
			if err != nil {
				fmt.Println("Unable to post token: ", err)
				ginutil.WriteAPIClientWriteError(i.c, err, "Unable to get existing token or create new token.")
				return wharfapi.Token{}, false
			}
		}
	}

	return token, true
}

func (i *azureImporter) getOrPostProviderWritesProblem(provider wharfapi.Provider) (wharfapi.Provider, bool) {
	var err error
	if provider.ProviderID != 0 {
		provider, err = i.wharf.GetProviderById(provider.ProviderID)
		if err != nil || provider.ProviderID == 0 {
			fmt.Printf("Unable to get provider. %+v", err)
			ginutil.WriteAPIClientReadError(i.c, err,
				fmt.Sprintf("Unable to get provider by ID %d", provider.ProviderID))
			return wharfapi.Provider{}, false
		}
	} else {
		provider, err = i.wharf.GetProvider(
			apiProviderName,
			provider.URL,
			provider.UploadURL,
			i.token.TokenID)
		if err != nil || provider.ProviderID == 0 {
			provider, err = i.wharf.PostProvider(
				wharfapi.Provider{
					Name:    apiProviderName,
					URL:     provider.URL,
					TokenID: i.token.TokenID})
			if err != nil {
				fmt.Println("Unable to post provider: ", err)
				ginutil.WriteAPIClientWriteError(i.c, err,
					fmt.Sprintf("Unable to get or create provider from %q.", provider.URL))
				return wharfapi.Provider{}, false
			}
		}
	}

	return provider, true
}

func (i *azureImporter) constructGitURL(groupName, projectName string) (string, error) {
	providerURL, err := url.Parse(i.provider.URL)

	if err != nil {
		fmt.Println("Unable to parse provider url: ", err)
		return "", err
	}

	const sshPort = 22
	gitURL := fmt.Sprintf("git@%s:%d/%s/%s/_git/%s", providerURL.Host, sshPort, groupName, projectName, projectName)
	return gitURL, nil
}
