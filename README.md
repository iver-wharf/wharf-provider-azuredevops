# Azure DevOps provider for Wharf

Import Wharf projects from Azure DevOps repositories. Mainly focused on
importing from self hosted Azure DevOps instances, importing from
dev.azure.com is not well tested.

## Components

- HTTP API using the [gin-gonic/gin](https://github.com/gin-gonic/gin)
  web framework.

- Swagger documentation generated using
  [swaggo/swag](https://github.com/swaggo/swag) and hosted using
  [swaggo/gin-swagger](https://github.com/swaggo/gin-swagger)

- Azure DevOps REST API accessed using good'ol `net/http` and `encoding/json`

## Development

1. Install Go 1.13 or later: <https://golang.org/>

2. Install the [swaggo/swag](https://github.com/swaggo/swag) CLI globally:

   ```sh
   # Run this outside of any Go module, including this repository, to not
   # have `go get` update the go.mod file.
   $ cd ..

   $ go get -u github.com/swaggo/swag
   ```

3. Generate the swaggo files (this has to be redone each time the swaggo
   documentation comments has been altered):

   ```sh
   # Navigate back to this repository
   $ cd wharf-provider-github

   # Generate the files into docs/
   $ swag
   ```

4. Start hacking with your favorite tool. For example VS Code, GoLand,
   Vim, Emacs, or whatnot.

---

Maintained by [Iver](https://www.iver.com/en).
Licensed under the [MIT license](./LICENSE).
