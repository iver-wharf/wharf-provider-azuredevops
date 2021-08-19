commit = $(shell git rev-parse HEAD)
version = latest

build: swag
	go build .
	@echo "Built binary found at ./wharf-provider-azuredevops or ./wharf-provider-azuredevops.exe"

docker:
	@echo docker build . \
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

swag:
	swag init --parseDependency --parseDepth 1

deps:
	cd .. && go get -u github.com/swaggo/swag
	go mod download
