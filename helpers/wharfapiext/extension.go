package wharfapiext

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"regexp"

	"github.com/iver-wharf/wharf-api-client-go/pkg/wharfapi"
	log "github.com/sirupsen/logrus"
)

type ExtClient struct {
	*wharfapi.Client
}

func (c ExtClient) CreateOrUpdate(data interface{}, method, dataName, endpoint string) (interface{}, error) {
	var newData = reflect.New(reflect.TypeOf(data)).Elem().Interface()
	body, err := json.Marshal(data)
	if err != nil {
		return newData, err
	}

	requestURL := fmt.Sprintf("%s/api%s", c.ApiUrl, endpoint)
	ioBody, err := doRequest(fmt.Sprintf("%s | %s", method, dataName), method, requestURL, body, c.AuthHeader)
	if err != nil {
		return newData, err
	}

	defer (*ioBody).Close()

	err = json.NewDecoder(*ioBody).Decode(&newData)
	if err != nil {
		return newData, err
	}

	return newData, nil
}

func (c ExtClient) PostBranch(branch wharfapi.Branch) (wharfapi.Branch, error) {
	data, err := c.CreateOrUpdate(branch, http.MethodPost, "BRANCH", "/branch")
	if err != nil {
		return wharfapi.Branch{}, err
	}

	return data.(wharfapi.Branch), nil
}

func (c ExtClient) PutBranches(branch []wharfapi.Branch) ([]wharfapi.Branch, error) {
	data, err := c.CreateOrUpdate(branch, http.MethodPut, "BRANCHES", "/branches")
	if err != nil {
		return []wharfapi.Branch{}, err
	}

	return data.([]wharfapi.Branch), nil
}

func (c ExtClient) PostToken(token wharfapi.Token) (wharfapi.Token, error) {
	data, err := c.CreateOrUpdate(token, http.MethodPost, "TOKEN", "/token")
	if err != nil {
		return wharfapi.Token{}, err
	}

	return data.(wharfapi.Token), nil
}

func (c ExtClient) PutProject(project wharfapi.Project) (wharfapi.Project, error) {
	data, err := c.CreateOrUpdate(project, http.MethodPut, "PROJECT", "/project")
	if err != nil {
		return wharfapi.Project{}, err
	}

	return data.(wharfapi.Project), nil
}

var redacted = "*REDACTED*"
var tokenPatternJSON = regexp.MustCompile(`("token"\s*:\s*"([a-zA-Z\d\s]+)")\s*`)
var tokenReplacementJSON = fmt.Sprintf(`"token":"%s"`, redacted)

func redactTokenInJSON(src string) string {
	if !tokenPatternJSON.MatchString(src) {
		return src
	}

	return tokenPatternJSON.ReplaceAllString(src, tokenReplacementJSON)
}

func redactTokenInURL(urlStr string) string {
	if urlStr == "" {
		return ""
	}

	uri, err := url.Parse(urlStr)
	if err != nil {
		log.WithError(err).Warningln("Unable to redact token from URL: parse URL")
		return ""
	}

	params, err := url.ParseQuery(uri.RawQuery)
	if err != nil {
		log.WithError(err).Warningln("Unable to redact token from URL: parse query")
		return ""
	}

	token := params.Get("Token")
	if token != "" {
		params.Set("Token", redacted)
	} else {
		token = params.Get("token")
		if token != "" {
			params.Set("token", redacted)
		}
	}

	uri.RawQuery = params.Encode()
	newURLStr := uri.String()

	sanitized, err := url.PathUnescape(newURLStr)
	if err != nil {
		log.WithError(err).WithField("new URL string", newURLStr).Warningln("Unable to redact token from URL: unescape path")
		return newURLStr
	}

	return sanitized
}

func doRequest(from string, method string, URLStr string, body []byte, authHeader string) (*io.ReadCloser, error) {
	log.WithFields(log.Fields{
		"method": method,
		"body":   redactTokenInJSON(string(body)),
		"url":    redactTokenInURL(URLStr),
	}).Debugln(from)

	req, err := http.NewRequest(method, URLStr, bytes.NewReader(body))
	if err != nil {
		log.WithError(err).Errorln("Unable to prepare http request")
		return nil, err
	}

	if authHeader != "" {
		req.Header.Add("Authorization", authHeader)
	}

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		log.WithError(err).Errorln("Unable to send http request")
		return nil, err
	}

	if response.StatusCode == http.StatusUnauthorized {
		response.Body.Close()
		log.WithField("response", response).Errorln("Unauthorized")
		realm := response.Header.Get("WWW-Authenticate")
		return nil, &wharfapi.AuthError{Realm: realm}
	}

	if response.StatusCode < 200 && response.StatusCode >= 300 {
		resp, err := ioutil.ReadAll(response.Body)
		response.Body.Close()
		if err == nil {
			log.WithFields(log.Fields{
				"status":        response.Status,
				"response body": string(resp)}).Debug("Got invalid status code")
		}
		return nil, fmt.Errorf("unexpected status code returned %v", response.StatusCode)
	}

	return &response.Body, nil
}
