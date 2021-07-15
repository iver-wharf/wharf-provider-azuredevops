package requests

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

// ConstructGetURL creates a GET request from a base URL, a map of query
// parameters, and a formatted path.
func ConstructGetURL(
	rawURL string, queries map[string][]string, format string, values ...interface{}) (*url.URL, error) {

	urlPath, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	urlPath.Path = fmt.Sprintf(format, values...)
	var q url.Values = queries
	urlPath.RawQuery = q.Encode()

	return urlPath, nil
}

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
