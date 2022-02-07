commit = $(shell git rev-parse HEAD)
version = latest

build: swag
	go build .
	@echo "Built binary found at ./wharf-provider-azuredevops or ./wharf-provider-azuredevops.exe"

test: swag
	go test -v ./...

docker:
	docker build . \
		--pull \
		-t "quay.io/iver-wharf/wharf-provider-azuredevops:latest" \
		-t "quay.io/iver-wharf/wharf-provider-azuredevops:$(version)" \
		--build-arg BUILD_VERSION="$(version)" \
		--build-arg BUILD_GIT_COMMIT="$(commit)" \
		--build-arg BUILD_DATE="$(shell date --iso-8601=seconds)"
	@echo ""
	@echo "Push the image by running:"
	@echo "docker push quay.io/iver-wharf/wharf-provider-azuredevops:latest"
ifneq "$(version)" "latest"
	@echo "docker push quay.io/iver-wharf/wharf-provider-azuredevops:$(version)"
endif

docker-run:
	docker run --rm -it quay.io/iver-wharf/wharf-provider-azuredevops:$(version)

serve: swag
	go run .

swag-force:
	swag init --parseDependency --parseDepth 2

swag:
ifeq ("$(wildcard docs/docs.go)","")
	swag init --parseDependency --parseDepth 2
else
ifeq ("$(filter $(MAKECMDGOALS),swag-force)","")
	@echo "-- Skipping 'swag init' because docs/docs.go exists."
	@echo "-- Run 'make' with additional target 'swag-force' to always run it."
endif
endif
	@# This comment silences warning "make: Nothing to be done for 'swag'."

deps:
	cd .. && go get -u github.com/swaggo/swag/cmd/swag@v1.7.1
	go mod download
