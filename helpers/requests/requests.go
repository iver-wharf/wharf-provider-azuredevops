package requests

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

// ConstructGetURL Constructs a URL to use in a GET request.
// Queries are passed as a map of string arrays
func ConstructGetURL(
	rawURL string, queries map[string][]string, format string, values ...interface{}) (*url.URL, error) {

	urlPath, err := url.Parse(rawURL)
	if err != nil {
		fmt.Printf("Unable to parse url %q", rawURL)
		return nil, err
	}

	urlPath.Path = fmt.Sprintf(format, values...)
	var q url.Values = queries
	urlPath.RawQuery = q.Encode()

	return urlPath, nil
}

// GetAndParseJSON Makes a request to urlPath with basic auth using user and token.
//	Attempts to parse the JSON response, returning it in "result" if successful.
func GetAndParseJSON(result interface{}, user, token string, urlPath *url.URL) error {
	body, err := getBodyFromRequest(user, token, urlPath.String())
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, &result)
	if err != nil {
		fmt.Println("Unable to unmarshal refs: ", err)
		return err
	}

	return nil
}

// GetAsString Makes a request to urlPath with basic auth using user and token.
// Returns the response as a string
func GetAsString(user, token string, urlPath *url.URL) (string, error) {
	body, err := getBodyFromRequest(user, token, urlPath.String())
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func getBodyFromRequest(user string, token string, url string) ([]byte, error) {
	fmt.Println("attempting to get from: ", url)
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