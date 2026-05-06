FROM golang:1.25-alpine AS build

ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /out/tc-server ./tc-server/cmd/tc-server \
	&& CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /out/tc-control ./tc-control/cmd/tc-control \
	&& CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /out/tc-worker ./tc-worker/cmd/tc-worker \
	&& CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /out/tcctl ./tcctl/cmd/tcctl

FROM alpine:3.22 AS runtime-base

RUN adduser -D -u 10001 touchconnect \
	&& mkdir -p /data /artifacts /skills \
	&& chown -R touchconnect:touchconnect /data /artifacts /skills

USER touchconnect
WORKDIR /workspace

FROM runtime-base AS tc-server

COPY --from=build /out/tc-server /usr/local/bin/tc-server

ENTRYPOINT ["tc-server"]

FROM runtime-base AS tc-control

COPY --from=build /out/tc-control /usr/local/bin/tc-control

ENTRYPOINT ["tc-control"]

FROM runtime-base AS dev-tools

COPY --from=build /out/tc-server /usr/local/bin/tc-server
COPY --from=build /out/tc-control /usr/local/bin/tc-control
COPY --from=build /out/tc-worker /usr/local/bin/tc-worker
COPY --from=build /out/tcctl /usr/local/bin/tcctl
