# syntax=docker/dockerfile:1
# Builds one of the four projects from its GitHub repository at a tag or branch, so the stack
# runs before every project publishes an image of its own.
#
#   docker build --build-arg PROJECT=airlock --build-arg REF=v0.1.0 .

FROM golang:1.27.4@sha256:205c7636d74e0ffbe3d007162df7d74f859f5a8c8322dfb9f907977686c126cf AS build
ARG PROJECT
ARG REF=main
# Extra packages to build next to cmd/<project>, such as transponder's demo/sim.
ARG EXTRA=
ADD --keep-git-dir=false https://github.com/raven-clown/${PROJECT}.git#${REF} /src
WORKDIR /src
RUN go mod download
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/ ./cmd/${PROJECT} ${EXTRA}

# Alpine rather than distroless: the health checks use wget, and the seed steps use sh.
FROM alpine:3.22@sha256:49340b346798875b5592e5d420241f2f9cfba5f47e62ebb1446a1b34734e898f
RUN mkdir -p /data /keys && chown 65532:65532 /data /keys
COPY --from=build /out/ /usr/local/bin/
USER 65532:65532
WORKDIR /data
