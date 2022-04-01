FROM golang:1.18.0-buster AS build
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY app/*.go ./
RUN go build -o gcp-idle-resources-metrics

FROM gcr.io/distroless/base-debian10
WORKDIR /
COPY --from=build /app/gcp-idle-resources-metrics /gcp-idle-resources-metrics
USER nonroot:nonroot
ENTRYPOINT ["/gcp-idle-resources-metrics"]
