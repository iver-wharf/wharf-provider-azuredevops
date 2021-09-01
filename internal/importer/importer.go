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
	"github.com/iver-wharf/wharf-core/pkg/logger"
	"github.com/iver-wharf/wharf-provider-azuredevops/internal/azureapi"
)

const (
	apiProviderName         = "azuredevops"
	buildDefinitionFileName = ".wharf-ci.yml"
)

var log = logger.NewScoped("IMPORTER")

// Importer is an interface for importing project data from a remote provider to
// the Wharf API.
//
// All of the functions will write a problem to the provided gin.Context when an
// error occurs.
type Importer interface {
	// InitWritesProblem gets/creates the specified token and provider from the Wharf API and
	// initializes the AzureAPI client.
	InitWritesProblem(token wharfapi.Token, provider wharfapi.Provider, c *gin.Context, client wharfapi.Client) bool
	// ImportRepositoryWritesProblem imports a given Azure DevOps repository
	// into Wharf.
	ImportRepositoryWritesProblem(orgName, projectNameOrID, repoNameOrID string) bool
	// ImportProjectWritesProblem imports all Azure DevOps repositories from a
	// given Azure DevOps project into Wharf.
	ImportProjectWritesProblem(orgName, projectNameOrID string) bool
	// ImportOrganizationWritesProblem imports all Azure DevOps repositories
	// from all projects found in an Azure DevOps organization into Wharf.
	ImportOrganizationWritesProblem(orgName string) bool
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

// NewAzureImporter creates a new azureImporter.
func NewAzureImporter(c *gin.Context, client *wharfapi.Client) Importer {
	return azureImporter{
		c:     c,
		wharf: client,
	}
}

func (i azureImporter) InitWritesProblem(token wharfapi.Token, provider wharfapi.Provider, c *gin.Context, client wharfapi.Client) bool {
	var ok bool
	i.token, ok = i.getOrPostTokenWritesProblem(token)
	if !ok {
		log.Error().Message("Failed to get or create token.")
		return false
	}
	log.Debug().
		WithUint("id", i.token.TokenID).
		Message("Token from DB.")

	i.provider, ok = i.getOrPostProviderWritesProblem(provider)
	if !ok {
		return false
	}
	log.Debug().
		WithUint("id", i.provider.ProviderID).
		WithString("name", i.provider.Name).
		WithString("url", i.provider.URL).
		Message("Provider from DB.")

	i.wharf = &client

	urlParsed, err := url.Parse(i.provider.URL)
	if err != nil {
		ginutil.WriteComposingProviderDataError(i.c, err,
			fmt.Sprintf("Unable parse the provider URL %q", i.provider.URL))
	}

	i.azure = &azureapi.Client{
		Context:       c,
		BaseURL:       i.provider.URL,
		BaseURLParsed: urlParsed,
		UserName:      i.token.UserName,
		Token:         i.token.Token,
	}

	return true
}

func (i azureImporter) ImportRepositoryWritesProblem(orgName, projectNameOrID, repoNameOrID string) bool {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	repo, ok := i.azure.GetRepositoryWritesProblem(orgName, projectNameOrID, repoNameOrID)
	if !ok {
		return false
	}

	return i.importKnownRepositoryWritesProblem(orgName, repo)
}

func (i azureImporter) ImportProjectWritesProblem(orgName, projectNameOrID string) bool {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	repos, ok := i.azure.GetRepositoriesWritesProblem(orgName, projectNameOrID)
	if !ok {
		return false
	}
	for _, repo := range repos {
		ok := i.importKnownRepositoryWritesProblem(orgName, repo)
		if !ok {
			return false
		}
	}
	return true
}

func (i azureImporter) ImportOrganizationWritesProblem(groupName string) bool {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	projects, ok := i.azure.GetProjectsWritesProblem(groupName)
	if !ok {
		return false
	}

	for _, project := range projects {
		ok := i.ImportProjectWritesProblem(groupName, project.Name)
		if !ok {
			return false
		}
	}
	return true
}

func (i azureImporter) importKnownRepositoryWritesProblem(orgName string, repo azureapi.Repository) bool {
	buildDef, ok := i.azure.GetFileWritesProblem(orgName, repo.Project.Name, repo.Name, buildDefinitionFileName)
	if !ok {
		return false
	}

	branches, ok := i.azure.GetRepositoryBranchesWritesProblem(orgName, repo.Project.Name, repo.Name)
	if !ok {
		return false
	}

	wharfProject, ok := i.importRepositoryWritesProblem(orgName, repo, buildDef)
	if !ok {
		return false
	}

	ok = i.importBranchesWritesProblem(repo.DefaultBranchRef, branches, wharfProject.ProjectID)

	return ok
}

func (i azureImporter) importRepositoryWritesProblem(orgName string, repo azureapi.Repository, buildDef string) (wharfapi.Project, bool) {
	projectInDB, err := i.createOrUpdateWharfProject(orgName, repo, buildDef)

	if err != nil {
		log.Error().
			WithError(err).
			WithString("org", orgName).
			WithString("project", repo.Project.Name).
			WithString("repo", repo.Name).
			Message("Unable to create project.")
		ginutil.WriteAPIClientWriteError(i.c, err,
			fmt.Sprintf("Unable to import repository %q from project %q in organization %q.",
				repo.Name, repo.Project.Name, orgName))
		return wharfapi.Project{}, false
	}

	return projectInDB, true
}

func (i azureImporter) importBranchesWritesProblem(defaultBranchRef string, branches []azureapi.Branch, wharfProjectID uint) bool {
	wharfBranches := make([]wharfapi.Branch, len(branches))
	for idx, branch := range branches {
		wharfBranches[idx] = wharfapi.Branch{
			Name:      branch.Name,
			ProjectID: wharfProjectID,
			Default:   branch.Ref == defaultBranchRef,
			TokenID:   i.token.TokenID,
		}
	}

	if _, err := i.wharf.PutBranches(wharfBranches); err != nil {
		log.Error().
			WithError(err).
			WithInt("branchesCount", len(branches)).
			WithUint("projectId", wharfProjectID).
			Message("Unable to replace branches for Wharf project.")
		ginutil.WriteAPIClientWriteError(i.c, err, fmt.Sprintf("Unable to replace branches for Wharf project with ID %d.", wharfProjectID))
		return false
	}

	return true
}

// createOrUpdateWharfProject tries to create a new Wharf project via the
// Wharf API.
//
// This contains backward compatibility by updating an existing Wharf project
// if found that was previously named using the v1 format:
// 	Group:   "{orgName}"
// 	Project: "{repo.Project.Name}"
//
// But now they need to be renamed to:
// 	Group:   "{orgName}/{repo.Project.Name}"
// 	Project: "{repo.Name}"
//
// This relies on the "cannot-change-group" being removed, as was done in
// wharf-api v4.2.0: https://github.com/iver-wharf/wharf-api/pull/55
func (i azureImporter) createOrUpdateWharfProject(orgName string, repo azureapi.Repository, buildDef string) (wharfapi.Project, error) {
	var project wharfapi.Project
	searchResults, err := i.wharf.SearchProject(wharfapi.Project{
		Name:       repo.Project.Name,
		GroupName:  orgName,
		ProviderID: i.provider.ProviderID,
	})
	if err != nil && len(searchResults) > 0 {
		project = searchResults[0]
	}
	project.Name = repo.Name
	project.TokenID = i.token.TokenID
	project.GroupName = fmt.Sprintf("%s/%s", orgName, repo.Project.Name)
	project.BuildDefinition = buildDef
	project.Description = repo.Project.Description
	project.ProviderID = i.provider.ProviderID
	project.GitURL = repo.SSHURL
	return i.wharf.PutProject(project)
}

func (i azureImporter) getTokenByIDWritesProblem(tokenID uint) (wharfapi.Token, bool) {
	token, err := i.wharf.GetTokenByID(tokenID)
	if err != nil || token.TokenID == 0 {
		log.Error().WithError(err).WithUint("tokenId", token.TokenID).Message("Unable to get token.")
		ginutil.WriteAPIClientReadError(i.c, err,
			fmt.Sprintf("Unable to get token by ID %d.", tokenID))
		return wharfapi.Token{}, false
	}

	return token, true
}

func (i azureImporter) getOrPostTokenWritesProblem(token wharfapi.Token) (wharfapi.Token, bool) {
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
				log.Error().WithError(err).Message("Unable to create token.")
				ginutil.WriteAPIClientWriteError(i.c, err, "Unable to get existing token or create new token.")
				return wharfapi.Token{}, false
			}
		}
	}

	return token, true
}

func (i azureImporter) getOrPostProviderWritesProblem(provider wharfapi.Provider) (wharfapi.Provider, bool) {
	var err error
	if provider.ProviderID != 0 {
		provider, err = i.wharf.GetProviderByID(provider.ProviderID)
		if err != nil || provider.ProviderID == 0 {
			log.Error().WithError(err).Message("Unable to get provider.")
			ginutil.WriteAPIClientReadError(i.c, err,
				fmt.Sprintf("Unable to get provider by ID %d", provider.ProviderID))
			return wharfapi.Provider{}, false
		}
	} else {
		// TODO: `provider` gets overridden, even if err == nil
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
				log.Error().WithError(err).Message("Unable to create provider.")
				ginutil.WriteAPIClientWriteError(i.c, err,
					fmt.Sprintf("Unable to get or create provider from %q.", provider.URL))
				return wharfapi.Provider{}, false
			}
		}
	}

	return provider, true
}
