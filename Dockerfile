#Build stage
FROM golang:1.20-alpine3.17 AS build-env

RUN apk --no-cache add build-base git

COPY . /srv
WORKDIR /srv
RUN make build

FROM alpine:3.17
LABEL maintainer="contact@forgejo.org"

COPY --from=build-env /srv/forgejo-runner /bin/forgejo-runner

ENTRYPOINT ["/bin/forgejo-runner"]
