FROM golang:1.16.4 AS build
WORKDIR /src
ENV GO111MODULE=on
RUN go get -u github.com/swaggo/swag/cmd/swag@v1.6.5
COPY . /src
ARG BUILD_VERSION="local docker"
ARG BUILD_GIT_COMMIT="HEAD"
ARG BUILD_REF="0"
RUN deploy/update-version.sh version.yaml \
		&& swag init \
		&& go get -t -d \
		&& CGO_ENABLED=0 go build -o main

FROM alpine:3.13.4 AS final
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /src/main ./
ENTRYPOINT ["/app/main"]
