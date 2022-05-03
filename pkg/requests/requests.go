package requests

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/iver-wharf/wharf-core/pkg/logger"
)

var log = logger.NewScoped("REQUESTS")

// GetUnmarshalJSON invokes a HTTP request with basic auth.
// On success the response body will be unmarshalled as JSON.
func GetUnmarshalJSON(result any, user, token string, urlPath *url.URL) error {
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
		return []byte{}, fmt.Errorf("unable to get: %w", err)
	}

	req.SetBasicAuth(user, token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return []byte{}, fmt.Errorf("unable to get: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return []byte{}, fmt.Errorf("unable to get: %w", newNon2xxStatusError(resp))
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error().WithError(err).WithStringer("url", urlPath).Message("Failed to read HTTP response body.")
		return []byte{}, fmt.Errorf("unable to get: %w", err)
	}

	return bodyBytes, nil
}
