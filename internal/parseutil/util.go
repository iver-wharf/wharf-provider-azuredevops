package parseutil

import "strings"

// ParseRepoRefParams parses group and project name data into organization,
// project, and repository names required for AzureDevOps.
//
// Example 1:
//    Input:
//           groupName="iver-wharf/wharf"
//           projectName="provider-azuredevops"
//   Output:
//           orgName="iver-wharf"
//       projectName="wharf"
//          repoName="provider-azuredevops"
func ParseRepoRefParams(wharfGroupName, wharfProjectName string) (azureOrgName, azureProjectName, azureRepoName string) {
	azureOrgName, azureProjectName = splitStringOnceRune(wharfGroupName, '/')
	if azureProjectName == "" {
		azureProjectName = wharfProjectName
		azureRepoName = ""
	} else {
		azureRepoName = wharfProjectName
	}
	return
}

func splitStringOnceRune(value string, delimiter rune) (a, b string) {
	const notFoundIndex = -1
	delimiterIndex := strings.IndexRune(value, delimiter)
	if delimiterIndex == notFoundIndex {
		a = value
		b = ""
		return
	}
	a = value[:delimiterIndex]
	b = value[delimiterIndex+1:] // +1 to skip the delimiter
	return
}
