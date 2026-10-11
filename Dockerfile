FROM golang:1.26.1-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0
RUN make build_server
RUN go install github.com/pressly/goose/v3/cmd/goose@v3.27.1

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 growscada
WORKDIR /app
COPY --from=build /src/bin/growscada_combined_server/ /app/
COPY --from=build /go/bin/goose /usr/local/bin/goose
COPY internal/server/infrastructure/postgres/migrations/ /migrations/
USER growscada
EXPOSE 8080 9090
ENTRYPOINT ["/app/growscada_combined_server"]
