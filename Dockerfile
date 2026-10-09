# Build SoroForge into a minimal image.
#
#   docker build -t soroforge .
#   docker run --rm -e DATABASE_URL=... soroforge list

FROM golang:1.26-alpine AS build

WORKDIR /src

# Copy the module files first so dependency download is cached independently of
# source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev

# CGO is not needed: pgx is pure Go. Disabling it produces a static binary that
# runs on a distroless base.
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/soroforge ./cmd/soroforge

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/soroforge /usr/local/bin/soroforge

# The signing key reaches this process through the environment or a mounted
# keystore file; nothing secret is baked into the image.
USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/soroforge"]
CMD ["--help"]
