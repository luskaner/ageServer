# syntax=docker/dockerfile:1

# Compile
FROM golang:1.27-alpine3.24 AS compiler
WORKDIR /app
COPY tools/server-docker/Dockerfile/server/go.work.template go.work
COPY common common
COPY battle-server-broadcast battle-server-broadcast
COPY server server
RUN mkdir build
RUN cp -r server/resources build/resources && rm -rf build/resources/windows && rm -rf build/resources/unix
RUN mkdir -p build/resources/certificates
RUN go build -ldflags="-s -w" -o build/server ./server
# Compress
FROM alpine:3.24 AS compressor
RUN apk add --no-cache upx
WORKDIR /app
COPY --from=compiler /app/build/server .
RUN upx --lzma server
# Execute
FROM alpine:3.24
# https
EXPOSE 443/tcp
# Announcement
EXPOSE 31978/udp
VOLUME /app/server/resources/config
VOLUME /app/server/resources/certificates
VOLUME /tmp/ageServer/battle-servers
WORKDIR /app/server
COPY --from=compiler /app/build/resources resources
COPY --from=compressor /app/server .
ENTRYPOINT ["./server"]