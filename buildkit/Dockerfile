# syntax = docker/dockerfile:1.4.3
FROM golang:1.19 as build
WORKDIR /go/src/app
ENV CGO_ENABLED=0
COPY --link . .
# Ensure we can reference astronomer libs
ENV GOPRIVATE github.com/astronomer
COPY image/gitconfig /root/.gitconfig
# Download public key for github.com
#RUN --mount=type=ssh mkdir -p -m 0600 /root/.ssh && ssh-keyscan github.com >> /root/.ssh/known_hosts
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install -v ./...

FROM scratch
LABEL moby.buildkit.frontend.network.none="true"
COPY --from=build /go/bin/astro-runtime-frontend  /
ENTRYPOINT ["/astro-runtime-frontend"]
