FROM golang:1.24.0-buster AS build
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY . ./
RUN  wget https://github.com/prometheus/promu/releases/download/v0.13.0/promu-0.13.0.linux-amd64.tar.gz && \
  tar -xvzf promu-0.13.0.linux-amd64.tar.gz && \
  mv promu-0.13.0.linux-amd64/promu /bin/

RUN promu build

FROM gcr.io/distroless/base-debian10
WORKDIR /
COPY --from=build /app/server /server
USER nonroot:nonroot
ENTRYPOINT ["/server"]
