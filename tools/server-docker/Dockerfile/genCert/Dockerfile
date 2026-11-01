# syntax=docker/dockerfile:1

# Compile
FROM golang:1.27-alpine3.24 AS compiler
WORKDIR /app
COPY tools/server-docker/Dockerfile/genCert/go.work.template go.work
COPY common common
COPY server-genCert server-genCert
RUN mkdir -p build/server/bin
RUN go build -ldflags="-s -w" -o build/server/bin/genCert ./server-genCert
# Compress
FROM alpine:3.24 AS compressor
RUN apk add --no-cache upx
WORKDIR /app
COPY --from=compiler /app/build/server .
RUN upx --lzma bin/genCert
# Execute
FROM scratch
VOLUME /app/server/resources/certificates
WORKDIR /app/server
COPY --from=compressor /app/bin bin
ENTRYPOINT ["./bin/genCert"]
