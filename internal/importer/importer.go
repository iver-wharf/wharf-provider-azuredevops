package importer

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/iver-wharf/wharf-api-client-go/v2/pkg/model/request"
	"github.com/iver-wharf/wharf-api-client-go/v2/pkg/model/response"
	"github.com/iver-wharf/wharf-api-client-go/v2/pkg/wharfapi"
	"github.com/iver-wharf/wharf-core/pkg/ginutil"
	"github.com/iver-wharf/wharf-core/pkg/logger"
	"github.com/iver-wharf/wharf-provider-azuredevops/internal/azureapi"
)

const (
	apiProviderName         = "azuredevops"
	buildDefinitionFileName = ".wharf-ci.yml"
)

var log = logger.NewScoped("IMPORTER")

// ReqToken is an alias for request.Token.
//
// Meant to increase readability when accessing the field of the same name
// in request.Token from a TokenData instance.
type ReqToken = request.Token

// TokenData is a struct combining request.Token with an ID,
// meant to support both creating and updating behavior with the same
// struct.
type TokenData struct {
	ReqToken
	ID uint
}

// ReqProvider is an alias for request.Provider.
//
// Here for symmetry with ReqToken.
type ReqProvider = request.Provider

// ProviderData is a struct combining request.Provider with an ID,
// meant to support both creating and updating behavior with the same
// struct.
type ProviderData struct {
	ReqProvider
	ID uint
}

// Importer is an interface for importing project data from a remote provider to
// the Wharf API.
//
// All of the functions will write a problem to the provided gin.Context when an
// error occurs.
type Importer interface {
	// InitWritesProblem gets/creates the specified token and provider from the Wharf API and
	// initializes the AzureAPI client.
	InitWritesProblem(tokenData TokenData, providerData ProviderData, c *gin.Context, client wharfapi.Client) bool
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
	resToken response.Token
	// retrieved from database
	resProvider response.Provider
}

// NewAzureImporter creates a new azureImporter.
func NewAzureImporter(c *gin.Context, client *wharfapi.Client) Importer {
	return &azureImporter{
		c:     c,
		wharf: client,
	}
}

func (i *azureImporter) InitWritesProblem(tokenData TokenData, providerData ProviderData, c *gin.Context, client wharfapi.Client) bool {
	var ok bool
	i.resToken, ok = i.getOrPostTokenWritesProblem(tokenData)
	if !ok {
		log.Error().Message("Failed to get or create token.")
		return false
	}
	log.Debug().
		WithUint("ID", i.resToken.TokenID).
		Message("Token from DB.")

	var providerWithTokenID = providerData
	providerWithTokenID.TokenID = i.resToken.TokenID
	i.resProvider, ok = i.getOrPostProviderWritesProblem(providerWithTokenID)
	if !ok {
		return false
	}
	log.Debug().
		WithUint("ID", i.resProvider.ProviderID).
		WithString("name", string(i.resProvider.Name)).
		WithString("url", i.resProvider.URL).
		Message("Provider from DB.")

	i.wharf = &client

	urlParsed, err := url.Parse(i.resProvider.URL)
	if err != nil {
		ginutil.WriteInvalidParamError(i.c, err, "provider.url",
			fmt.Sprintf("Unable parse the provider URL %q.", i.resProvider.URL))
		return false
	}

	i.azure = &azureapi.Client{
		Context:       c,
		BaseURL:       i.resProvider.URL,
		BaseURLParsed: urlParsed,
		UserName:      i.resToken.UserName,
		Token:         i.resToken.Token,
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

func (i *azureImporter) importRepositoryWritesProblem(orgName string, repo azureapi.Repository, buildDef string) (response.Project, bool) {
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
		return response.Project{}, false
	}

	return projectInDB, true
}

func (i *azureImporter) importBranchesWritesProblem(defaultBranchRef string, branches []azureapi.Branch, wharfProjectID uint) bool {
	for _, branch := range branches {
		wharfBranch := request.Branch{
			Name:    branch.Name,
			Default: branch.Ref == defaultBranchRef,
		}

		if _, err := i.wharf.CreateProjectBranch(wharfProjectID, wharfBranch); err != nil {
			log.Error().
				WithError(err).
				WithInt("branchesCount", len(branches)).
				WithUint("projectId", wharfProjectID).
				Message("Unable to replace branches for Wharf project.")
			ginutil.WriteAPIClientWriteError(i.c, err, fmt.Sprintf("Unable to replace branches for Wharf project with ID %d.", wharfProjectID))
			return false
		}
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
func (i *azureImporter) createOrUpdateWharfProject(orgName string, repo azureapi.Repository, buildDef string) (response.Project, error) {
	groupName := fmt.Sprintf("%s/%s", orgName, repo.Project.Name)

	var existingProject response.Project
	search := wharfapi.ProjectSearch{
		Name:       &repo.Name,
		GroupName:  &groupName,
		ProviderID: &i.resProvider.ProviderID,
	}
	searchResults, err := i.wharf.GetProjectList(search)
	if err != nil {
		log.Error().
			WithError(err).
			WithString("name", *search.Name).
			WithString("groupName", *search.GroupName).
			WithUint("providerId", *search.ProviderID).
			Message("Unable to search for existing project.")
		return existingProject, err
	}
	if len(searchResults.List) > 0 {
		existingProject = searchResults.List[0]
		updatedProject := request.ProjectUpdate{
			Name:            repo.Name,
			TokenID:         i.resToken.TokenID,
			GroupName:       groupName,
			BuildDefinition: buildDef,
			Description:     repo.Project.Description,
			ProviderID:      i.resProvider.ProviderID,
			GitURL:          repo.SSHURL,
		}
		return i.wharf.UpdateProject(existingProject.ProjectID, updatedProject)
	}

	createdProject, err := i.wharf.CreateProject(request.Project{
		Name:            repo.Name,
		TokenID:         i.resToken.TokenID,
		GroupName:       groupName,
		BuildDefinition: buildDef,
		Description:     repo.Project.Description,
		ProviderID:      i.resProvider.ProviderID,
		GitURL:          repo.SSHURL,
		RemoteProjectID: repo.Project.ID,
	})

	if err != nil {
		log.Error().
			WithError(err).
			WithString("name", repo.Project.Name).
			WithString("groupName", groupName).
			WithString("gitURL", repo.SSHURL).
			WithUint("providerId", *search.ProviderID).
			Message("Unable to create project.")
		return response.Project{}, err
	}

	return createdProject, nil
}

func (i *azureImporter) getOrPostTokenWritesProblem(tokenData TokenData) (response.Token, bool) {
	if tokenData.ID != 0 {
		dbToken, err := i.wharf.GetToken(tokenData.ID)
		if err != nil {
			log.Error().
				WithError(err).
				WithUint("ID", tokenData.ID).
				Message("Unable to get token by ID.")
			ginutil.WriteAPIClientReadError(i.c, err,
				fmt.Sprintf("Unable to get token by ID %d.", tokenData.ID))
			return response.Token{}, false
		}
		return dbToken, true
	}

	if tokenData.UserName == "" && tokenData.Token == "" {
		err := errors.New("both token and user were empty")
		ginutil.WriteInvalidParamError(i.c, err, "token",
			"Unable to create token when both user and token are empty.")
		return response.Token{}, false
	}

	search := wharfapi.TokenSearch{
		UserName: &tokenData.UserName,
	}
	searchResults, err := i.wharf.GetTokenList(search)
	if err != nil || len(searchResults.List) == 0 {
		log.Warn().
			WithError(err).
			WithInt("tokensFound", len(searchResults.List)).
			Message("Unable to get token. Will try to create one instead.")
		createdToken, err := i.wharf.CreateToken(request.Token{
			Token:      tokenData.Token,
			UserName:   tokenData.UserName,
			ProviderID: i.resProvider.ProviderID,
		})
		if err != nil {
			log.Error().WithError(err).Message("Unable to create token.")
			ginutil.WriteAPIClientWriteError(i.c, err, "Unable to create new token.")
			return response.Token{}, false
		}
		return createdToken, true
	}

	var foundToken response.Token
	var found bool
	for _, t := range searchResults.List {
		if t.Token == tokenData.Token {
			foundToken = t
			found = true
			break
		}
	}

	return foundToken, found
}

func (i *azureImporter) getOrPostProviderWritesProblem(providerData ProviderData) (response.Provider, bool) {
	if providerData.ID != 0 {
		dbProvider, err := i.wharf.GetProvider(providerData.ID)
		if err != nil {
			log.Error().
				WithError(err).
				WithUint("providerId", providerData.ID).
				Message("Unable to get provider by ID.")
			ginutil.WriteAPIClientReadError(i.c, err,
				fmt.Sprintf("Unable to get provider by ID %d", providerData.ID))
			return response.Provider{}, false
		}
		log.Debug().WithUint("providerId", dbProvider.ProviderID).
			Message("Got existing provider from DB.")
		return dbProvider, true
	}

	providerName := string(providerData.Name)
	search := wharfapi.ProviderSearch{
		Name: &providerName,
		URL:  &providerData.URL,
	}
	searchResults, err := i.wharf.GetProviderList(search)

	if err == nil {
		for _, p := range searchResults.List {
			if p.URL == providerData.URL {
				return p, true
			}
		}
	}

	log.Warn().
		WithError(err).
		WithInt("providersFound", len(searchResults.List)).
		Message("Unable to get provider. Will try to create one instead.")
	createdProvider, err := i.wharf.CreateProvider(request.Provider{
		Name:    request.ProviderName(providerData.Name),
		URL:     providerData.URL,
		TokenID: providerData.TokenID,
	})
	if err != nil {
		log.Error().WithError(err).Message("Unable to create provider.")
		ginutil.WriteAPIClientWriteError(i.c, err,
			fmt.Sprintf("Unable to get or create provider from %q.", providerData.URL))
		return response.Provider{}, false
	}
	return createdProvider, true
}
