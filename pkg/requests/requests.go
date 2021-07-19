package requests

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

const (
	apiRepositories = "_apis/git/repositories"
	apiProjects     = "_apis/projects"
	itemsPath       = "items"
	refsPath        = "refs"
)

// GetAndParseJSON invokes a HTTP request with basic auth.
// On success the response body will be unmarshalled as JSON.
func GetAndParseJSON(result interface{}, user, token string, urlPath *url.URL) error {
	body, err := getBodyFromRequest(user, token, urlPath)
	if err != nil {
		return err
	}
	err = json.Unmarshal(body, &result)

	return err
}

// GetAsString invokes a HTTP request with basic auth.
// Returns the response as a string.
func GetAsString(user, token string, urlPath *url.URL) (string, error) {
	body, err := getBodyFromRequest(user, token, urlPath)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func getBodyFromRequest(user string, token string, urlPath *url.URL) ([]byte, error) {
	url := urlPath.String()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return []byte{}, err
	}

	req.SetBasicAuth(user, token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return []byte{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return []byte{}, fmt.Errorf("unable to get. status code: %d", resp.StatusCode)
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return []byte{}, err
	}

	return bodyBytes, nil
}

// NewGetRepositories constructs a GET request for getting repositories.
func NewGetRepositories(rawURL, groupName, projectName string) (*url.URL, error) {
	urlPath, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	urlPath.Path = fmt.Sprintf("%s/%s/%s", groupName, projectName, apiRepositories)

	q := url.Values{}
	q.Add("api-version", "5.0")
	urlPath.RawQuery = q.Encode()

	return urlPath, nil
}

// NewGetFile constructs a GET request for getting a file from a repository.
func NewGetFile(rawURL, groupName, projectName, filePath string) (*url.URL, error) {
	urlPath, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	urlPath.Path = fmt.Sprintf("%s/%s/%s/%s/%s",
		groupName, projectName,
		apiRepositories, projectName,
		itemsPath)

	q := url.Values{}
	q.Add("scopePath", fmt.Sprintf("/%s", filePath))
	urlPath.RawQuery = q.Encode()

	return urlPath, nil
}

// NewGetProject constructs a GET request for getting a project.
func NewGetProject(rawURL, groupName, projectName string) (*url.URL, error) {
	urlPath, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	urlPath.Path = fmt.Sprintf("%s/%s/%s", groupName, apiProjects, projectName)

	q := url.Values{}
	q.Add("api-version", "5.0")
	urlPath.RawQuery = q.Encode()

	return urlPath, nil
}

// NewGetProjects constructs a GET request for getting all projects from a group.
func NewGetProjects(rawURL, groupName string) (*url.URL, error) {
	urlPath, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	urlPath.Path = fmt.Sprintf("%s/%s", groupName, apiProjects)

	q := url.Values{}
	q.Add("api-version", "5.0")
	urlPath.RawQuery = q.Encode()

	return urlPath, nil
}

// NewGetGitRefs constructs a GET request for getting git refs from a project.
func NewGetGitRefs(rawURL, groupName, projectName, refsFilter string) (*url.URL, error) {
	urlPath, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	urlPath.Path = fmt.Sprintf("%s/%s/%s/%s/%s",
		groupName, projectName,
		apiRepositories, projectName,
		refsPath)

	q := url.Values{}
	q.Add("api-version", "5.0")
	q.Add("filter", refsFilter)
	urlPath.RawQuery = q.Encode()

	return urlPath, nil
}
