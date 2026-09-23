ARG GO_VERSION=1.27.1
FROM golang:${GO_VERSION}-trixie AS build
RUN apt-get update && apt-get install -y --no-install-recommends libgdal-dev libproj-dev pkg-config g++ && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=1 PROJ_NETWORK=OFF
RUN go test ./... && go build -buildvcs=false -trimpath -o /out/eoo-aoo ./cmd/eoo-aoo

FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends gdal-bin ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/eoo-aoo /usr/local/bin/eoo-aoo
ENV PROJ_NETWORK=OFF
USER 65532:65532
WORKDIR /data
EXPOSE 8080
ENTRYPOINT ["eoo-aoo"]
CMD ["serve", "--listen", "0.0.0.0:8080"]
