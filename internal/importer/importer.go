package importer

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/iver-wharf/wharf-api-client-go/pkg/wharfapi"
	"github.com/iver-wharf/wharf-core/pkg/ginutil"
	"github.com/iver-wharf/wharf-core/pkg/logger"
	"github.com/iver-wharf/wharf-provider-azuredevops/internal/azureapi"
	"github.com/iver-wharf/wharf-provider-azuredevops/internal/parseutil"
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
	return &azureImporter{
		c:     c,
		wharf: client,
	}
}

func (i *azureImporter) InitWritesProblem(token wharfapi.Token, provider wharfapi.Provider, c *gin.Context, client wharfapi.Client) bool {
	var ok bool
	i.token, ok = i.getOrPostTokenWritesProblem(token)
	if !ok {
		log.Error().Message("Failed to get or create token.")
		return false
	}
	log.Debug().
		WithUint("id", i.token.TokenID).
		Message("Token from DB.")

	var providerWithTokenRef = provider
	providerWithTokenRef.TokenID = i.token.TokenID
	i.provider, ok = i.getOrPostProviderWritesProblem(providerWithTokenRef)
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
		ginutil.WriteInvalidParamError(i.c, err, "provider.url",
			fmt.Sprintf("Unable parse the provider URL %q.", i.provider.URL))
		return false
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

func (i *azureImporter) ImportRepositoryWritesProblem(orgName, projectNameOrID, repoNameOrID string) bool {
	repo, ok := i.azure.GetRepositoryWritesProblem(orgName, projectNameOrID, repoNameOrID)
	if !ok {
		return false
	}

	return i.importKnownRepositoryWritesProblem(orgName, repo)
}

func (i *azureImporter) ImportProjectWritesProblem(orgName, projectNameOrID string) bool {
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

func (i *azureImporter) ImportOrganizationWritesProblem(groupName string) bool {
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

func (i *azureImporter) RefreshRepositoryWritesProblem(projectID uint) bool {
	project, err := i.wharf.GetProjectByID(projectID)
	if err != nil {
		ginutil.WriteAPIClientReadError(i.c, err,
			fmt.Sprintf("Unable to get project with ID %d from Wharf API.", projectID))
		return false
	}

	orgName, projectName, repoNameOrID := parseutil.ParseRepoRefParams(project.GroupName, project.Name)
	if project.RemoteProjectID != "" {
		repoNameOrID = project.RemoteProjectID
	}

	return i.ImportRepositoryWritesProblem(orgName, projectName, repoNameOrID)
}

func (i *azureImporter) importKnownRepositoryWritesProblem(orgName string, repo azureapi.Repository) bool {
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

func (i *azureImporter) importRepositoryWritesProblem(orgName string, repo azureapi.Repository, buildDef string) (wharfapi.Project, bool) {
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

func (i *azureImporter) importBranchesWritesProblem(defaultBranchRef string, branches []azureapi.Branch, wharfProjectID uint) bool {
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
func (i *azureImporter) createOrUpdateWharfProject(orgName string, repo azureapi.Repository, buildDef string) (wharfapi.Project, error) {
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

func (i *azureImporter) getOrPostTokenWritesProblem(token wharfapi.Token) (wharfapi.Token, bool) {
	if token.TokenID != 0 {
		dbToken, err := i.wharf.GetTokenByID(token.TokenID)
		if err != nil {
			log.Error().
				WithError(err).
				WithUint("tokenId", token.TokenID).
				Message("Unable to get token by ID.")
			ginutil.WriteAPIClientReadError(i.c, err,
				fmt.Sprintf("Unable to get token by ID %d.", token.TokenID))
			return wharfapi.Token{}, false
		}
		return dbToken, true
	}

	if token.UserName == "" && token.Token == "" {
		err := errors.New("both token and user were empty")
		ginutil.WriteInvalidParamError(i.c, err, "token",
			"Unable to create token when both user and token are empty.")
		return wharfapi.Token{}, false
	}

	searchResults, err := i.wharf.SearchToken(token)
	if err != nil || len(searchResults) == 0 {
		log.Warn().
			WithError(err).
			WithInt("tokensFound", len(searchResults)).
			Message("Unable to get token. Will try to create one instead.")
		createdToken, err := i.wharf.PostToken(token)
		if err != nil {
			log.Error().WithError(err).Message("Unable to create token.")
			ginutil.WriteAPIClientWriteError(i.c, err, "Unable to create new token.")
			return wharfapi.Token{}, false
		}
		return createdToken, true
	}

	return searchResults[0], true
}

func (i *azureImporter) getOrPostProviderWritesProblem(provider wharfapi.Provider) (wharfapi.Provider, bool) {
	if provider.ProviderID != 0 {
		dbProvider, err := i.wharf.GetProviderByID(provider.ProviderID)
		if err != nil {
			log.Error().
				WithError(err).
				WithUint("providerId", provider.ProviderID).
				Message("Unable to get provider by ID.")
			ginutil.WriteAPIClientReadError(i.c, err,
				fmt.Sprintf("Unable to get provider by ID %d", provider.ProviderID))
			return wharfapi.Provider{}, false
		}
		return dbProvider, true
	}

	searchResults, err := i.wharf.SearchProvider(provider)
	if err != nil || len(searchResults) == 0 {
		log.Warn().
			WithError(err).
			WithInt("providersFound", len(searchResults)).
			Message("Unable to get provider. Will try to create one instead.")
		createdProvider, err := i.wharf.PostProvider(provider)
		if err != nil {
			log.Error().WithError(err).Message("Unable to create provider.")
			ginutil.WriteAPIClientWriteError(i.c, err,
				fmt.Sprintf("Unable to get or create provider from %q.", provider.URL))
			return wharfapi.Provider{}, false
		}
		return createdProvider, true
	}

	return searchResults[0], true
}
