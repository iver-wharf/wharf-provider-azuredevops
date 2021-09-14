# Wharf AzureDevOps plugin changelog

This project tries to follow [SemVer 2.0.0](https://semver.org/).

<!--
	When composing new changes to this list, try to follow convention.

	The WIP release shall be updated just before adding the Git tag.
	From (WIP) to (YYYY-MM-DD), ex: (2021-02-09) for 9th of Febuary, 2021

	A good source on conventions can be found here:
	https://changelog.md/
-->

## v2.1.0 (WIP)

- Changed version of `github.com/iver-wharf/wharf-api-client-go`
  from v1.3.1 -> v1.4.0. (#28)

- Changed version of `github.com/iver-wharf/wharf-core` from v1.1.0 -> v1.2.0.
  (#28)

- Removed `internal/httputils`, which was moved to
  `github.com/iver-wharf/wharf-core/pkg/cacertutil`. (#28)

## v2.0.1 (2021-09-10)

- Changed version of Docker base images, relying on "latest" patch version:

  - Alpine: 3.14.0 -> 3.14 (#35)
  - Golang: 1.16.5 -> 1.16 (#35)

## v2.0.0 (2021-09-09)

- BREAKING: Changed import procedure to import each Azure DevOps Git repository
  as its own Wharf project, compared to before where it imported each
  Azure DevOps project as its own Wharf project. (#31)

- BREAKING: Changed name format of imported Wharf projects.

  - Before v2.0.0:

    - Wharf group name: `{azure org name}`
    - Wharf project name: `{azure project name}`

  - Since v2.0.0:

    - Wharf group name: `{azure org name}/{azure project name}`
    - Wharf project name: `{azure Git repo name}`

  There are migrations in place to try and rename Wharf projects imported via
  wharf-provider-azuredevops before v2.0.0, but that also requires the use of
  wharf-api v4.2.0 or higher (see: <https://github.com/iver-wharf/wharf-api/pull/55>).

  This may break your builds! If you rely on the Wharf group or project names
  in your `.wharf-ci.yml` build pipeline then you need to update those
  accordingly. Recommended to use the built-in variables `REPO_GROUP` and
  `REPO_NAME` throughout your build pipeline instead (see: <https://iver-wharf.github.io/#/usage-wharfyml/variables/built-in-variables?id=repo_group>).

- BREAKING: Added a config for skipping TLS certificate chain verification to
  make it opt-in, where it always skipped the TLS certificates before. (#33)

  While skipping this verification is heavily discouraged, if you truly do
  rely on it then this can be re-enabled by setting either the YAML
  configuration value `ca.insecureSkipVerify` or the environment variable
  `WHARF_CA_INSECURESKIPVERIFY` to `true`.

- Fixed Git SSH URL when importing from <https://dev.azure.com>. (#31)

- Fixed duplicate token and provider creation in wharf-api. It will now reuse
  existing tokens and providers from the wharf-api appropriately. (#31)

- Changed to return IETF RFC-7807 compatible problem responses on failures
  instead of solely JSON-formatted strings. (#14)

- Changed version of `github.com/iver-wharf/wharf-core` from v0.0.0 -> v1.1.0.
  (#14, #23)

- Changed version of `github.com/iver-wharf/wharf-api-client-go`
  from v1.2.0 -> v1.3.1. (#26)

- Added Makefile to simplify building and developing the project locally.
  (#21, #22, #23)

- Added logging and custom exit code when app fails to bind the IP address and
  port. (#23)

- Added configs via wharf-core/pkg/config. Now supports both environment vars
  and YAML config files. (#23)

- Added possibility to load self-signed certs into the Go `http.DefaultClient`
  via the new config `ca.certsFile` or environment variable
  `WHARF_CA_CERTSFILE`, on top of the system's/OS's cert store. (#23)

- Added support for the TZ environment variable (setting timezones ex.
  `"Europe/Stockholm"`) through the tzdata package. (#20)

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
