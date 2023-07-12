#Build stage
# Switch back to 1.20 instead of 1.20.5 when https://github.com/nektos/act/issues/1908 is resolved
FROM golang:1.20.5-alpine3.18 AS build-env

RUN apk --no-cache add build-base git

COPY . /srv
WORKDIR /srv
RUN make build

FROM alpine:3.18
LABEL maintainer="contact@forgejo.org"

COPY --from=build-env /srv/forgejo-runner /bin/forgejo-runner

ENTRYPOINT ["/bin/forgejo-runner"]
