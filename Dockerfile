FROM golang:1.13.4 AS build
WORKDIR /src
ENV GO111MODULE=on
RUN go get -u github.com/swaggo/swag/cmd/swag@v1.6.5
COPY . /src
RUN swag init && CGO_ENABLED=0 go build -o main

FROM alpine:3.13.4 AS final
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /src/main ./
ENTRYPOINT ["/app/main"]
