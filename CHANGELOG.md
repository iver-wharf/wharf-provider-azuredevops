# Wharf AzureDevOps plugin changelog

This project tries to follow [SemVer 2.0.0](https://semver.org/).

<!--
	When composing new changes to this list, try to follow convention.

	The WIP release shall be updated just before adding the Git tag.
	From (WIP) to (YYYY-MM-DD), ex: (2021-02-09) for 9th of Febuary, 2021

	A good source on conventions can be found here:
	https://changelog.md/
-->

## v1.3.0 (WIP)

- Changed to return IETF RFC-7807 compatible problem responses on failures
  instead of solely JSON-formatted strings. (#14)

- Updated wharf-core from v0.0.0 -> v1.0.0. (#14)

- Added Makefile to simplify building and developing the project locally. (#21)

## v1.2.0 (2021-07-12)

- Added environment variable `BIND_ADDRESS` for setting bind address and port,
  which defaults to `0.0.0.0:8080` when left unset. (#11)

- Added endpoint `GET /version` that returns an object of version data of the
  API itself. (#3)

- Added Swagger spec metadata such as version that equals the version of the
  API, contact information, and license. (#3)

- Changed module Go version from v1.13 to v1.16. (#3)

- Changed version of Docker base images:

  - Alpine: 3.13.4 -> 3.14.0 (#12, #15)
  - Golang: 1.16.4 -> 1.16.5 (#15)

## v1.1.1 (2021-04-09)

- Added CHANGELOG.md to repository. (!17)

- Changed to use new open sourced Wharf API client
  [github.com/iver-wharf/wharf-api-client-go](https://github.com/iver-wharf/wharf-api-client-go)
  and bumped said package version from v1.1.0 to v1.2.0. (!18)

- Added `.dockerignore` to make Docker builds agnostic to wether you've ran
  `swag init` locally. (!19)

- Changed base Docker image to be `alpine:3.13.4` instead of `scratch` to get
  certificates from the Alpine package manager, APK, instead of embedding a
  list of certificates inside the repository. (#1)

## v1.1.0 (2021-01-07)

- Changed version of Wharf API Go client, from v0.1.5 to v1.1.0, that contained
  a lot of refactors in type and package name changes. (!15, !16)

## v1.0.0 (2020-11-27)

- Removed groups table, a reflection of the changes from the API v1.0.0. (!14)
- Changed version of Wharf API Go client from v0.1.3 to v0.1.4. (!13)

## v0.8.0 (2020-10-23)

- Changed all types to be named `AzureDevOps-` instead of `Tfs-`. (!10)

- Changed all endpoints to begin with `/import/azuredevops` instead of
  `/import/tfs`. (!11)

## v0.7.8 (2020-05-07)

- Changed version of Wharf API Go client from v0.1.1 to v0.1.3. (!9)

## v0.7.7 (2020-04-30)

- *Version bump.*

## v0.7.6 (2020-04-30)

- Added TFS webhook endpoint
  `POST /import/tfs/triggers/{projectid}/pr/created`.  (!8)

## v0.7.5 (2020-04-08)

- Fixed regression from !4, as branches are prefixed with `refs/heads/`, and not
  `ref/heads/`. (!7)

## v0.7.4 (2020-04-07)

- Fixed invalid URL construction by composing URLs via the `url` package instead
  of `fmt.Sprintf`. (!6)

## v0.7.3 (2020-04-07)

- Fixed regressions introduced in !4 regarding `fmt.Sprintf` formatting and an
  invalid return type. (!5)

## v0.7.2 (2020-04-06)

- Fixed branches being prefixed with `ref/heads/`. (!4)

## v0.7.1 (2020-03-11)

- Added supplying of `git_url` value when importing a project. (!3)

## v0.7.0 (2020-01-30)

- *Version bump.*

## v0.6.0 (2020-01-30)

- Changed Docker build to use Go modules instead of referencing each `.go`
  file explicitly. (db7f2ec5)

## v0.5.5 (2020-01-22)

- Added repo, as extracted from previous mono-repo. (08353015)

- Added `go.mod` & `go.sum`. (90853db4)

- Added `.wharf-ci.yml`. (e0259aff)

- Changed Docker build to use Go modules via `GO111MODULE=on` environment
  variable. (be84491b)

- Fixed Docker build to use `go.mod` instead of explicit references.
  (3ebebafd, e0259aff)
