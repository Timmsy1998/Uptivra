FROM golang:1.27.2-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY api ./api
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /uptivra ./cmd/uptivra

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -g 10001 app && adduser -D -u 10001 -G app app
COPY --from=build /uptivra /usr/local/bin/uptivra
USER app
EXPOSE 8080
ENTRYPOINT ["uptivra"]
CMD ["serve"]
