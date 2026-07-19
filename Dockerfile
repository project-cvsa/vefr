FROM golang:1.22-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY . ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/vefr ./cmd/proxy

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/vefr /usr/local/bin/vefr
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/vefr"]
CMD ["-config", "/etc/vefr/config.toml"]
