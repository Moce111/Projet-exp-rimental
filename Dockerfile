FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/sound-web ./cmd/webserver

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=build /out/sound-web /app/sound-web
COPY web /app/web
EXPOSE 8080
ENTRYPOINT ["/app/sound-web"]
