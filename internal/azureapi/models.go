package azureapi

// Branch represents branch data retrieved from Azure DevOps.
type Branch struct {
	Name          string
	Ref           string
	DefaultBranch bool
}

// Project represents project data retrieved from Azure DevOps.
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	State       string `json:"state"`
	Revision    int64  `json:"revision"`
	Visibility  string `json:"visibility"`
}

// PullRequestEvent represents a pull request event.
type PullRequestEvent struct {
	EventType string `json:"eventType" example:"git.pullrequest.created"`
	Resource  struct {
		PullRequestID uint   `json:"pullRequestId" example:"1"`
		SourceRefName string `json:"sourceRefName" example:"refs/heads/master"`
	}
}

// Repository represents repository data retrieved from Azure DevOps.
type Repository struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	URL              string  `json:"url"`
	Project          Project `json:"project"`
	DefaultBranchRef string  `json:"defaultBranch"`
	Size             int64   `json:"size"`
	RemoteURL        string  `json:"remoteUrl"`
	SSHURL           string  `json:"sshUrl"`
}

type creator struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	URL         string `json:"url"`
	UniqueName  string `json:"uniqueName"`
	ImageURL    string `json:"imageUrl"`
	Descriptor  string `json:"descriptor"`
}
