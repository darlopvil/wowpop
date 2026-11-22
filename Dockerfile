# syntax=docker/dockerfile:1

# ---------- build ----------
FROM golang:1.26-alpine AS build

RUN apk add --no-cache ca-certificates

WORKDIR /src

# Cached layer: only rebuilt when the dependencies change. The wildcard
# allows go.sum to be missing while there are no external dependencies.
COPY go.* ./
RUN go mod download

# templates/ and static/ are embedded in the binary with //go:embed
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /wowpop .

# ---------- runtime ----------
FROM scratch

# Needed for outbound TLS to Wallapop
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /wowpop /wowpop

USER 65534:65534
EXPOSE 8080

ENTRYPOINT ["/wowpop"]
