# syntax=docker/dockerfile:1

# Runner
FROM alpine:3.24 AS runner
RUN apk add --no-cache wine \
 && WINEPREFIX=/wine WINEARCH=win64 WINEDEBUG=-all DISPLAY="" \
    wineboot --init 2>/dev/null || true \
 && rm -rf /wine/drive_c/windows/syswow64 \
           /wine/drive_c/windows/twain_32 \
           /wine/drive_c/windows/system32/gecko \
           /wine/drive_c/windows/Microsoft.NET \
           /wine/drive_c/windows/explorer.exe \
           /wine/drive_c/windows/winhlp32.exe \
           /wine/drive_c/windows/winsxs
ENV WINEPREFIX=/wine \
    WINEDEBUG=-all \
    WINEARCH=win64 \
    DISPLAY=""
# Compile
FROM golang:1.27-alpine3.24 AS compiler
WORKDIR /app
COPY tools/server-docker/Dockerfile/battle-server-manager/go.work.template go.work
COPY common common
COPY battle-server-manager battle-server-manager
COPY battle-server-broadcast battle-server-broadcast
RUN mkdir -p build/resources/
RUN cp battle-server-manager/resources/config.game.toml build/resources/config.age1.toml
RUN cp battle-server-manager/resources/config.game.toml build/resources/config.age2.toml
RUN cp battle-server-manager/resources/config.game.toml build/resources/config.age3.toml
RUN cp battle-server-manager/resources/config.game.toml build/resources/config.age4.toml
RUN cp battle-server-manager/resources/config.game.toml build/resources/config.athens.toml
RUN go build -ldflags="-s -w" -o build/battle-server-manager ./battle-server-manager
# Compress
FROM alpine:3.24 AS compressor
RUN apk add --no-cache upx
WORKDIR /app
COPY --from=compiler /app/build/battle-server-manager .
RUN upx --lzma battle-server-manager
# Execute
FROM runner
VOLUME /app/battle-server-manager/resources
VOLUME /app/server/resources/certificates
VOLUME /tmp/ageServer/battle-servers
WORKDIR /app/battle-server-manager
COPY --from=compiler /app/build/resources resources
COPY --from=compressor /app/battle-server-manager .
ENV AGESERVER_BATTLE_SERVER_MANAGER_Ports_Bs=27012
ENV AGESERVER_BATTLE_SERVER_MANAGER_Ports_WebSocket=27112
ENV AGESERVER_BATTLE_SERVER_MANAGER_Ports_OutOfBand=27212
ENV AGESERVER_BATTLE_SERVER_MANAGER_CertsPath=/app/server/resources/certificates
ENV AGESERVER_BATTLE_SERVER_MANAGER_Executable_Path=/app/BattleServer.exe
EXPOSE 27012/tcp
EXPOSE 27112/tcp
EXPOSE 27212/tcp
ENTRYPOINT ["./battle-server-manager"]