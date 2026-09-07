ARG POSTGRES_IMAGE=postgres:18-alpine
FROM golang:1.26-alpine AS age-builder
ARG AGE_VERSION=v1.3.1
RUN CGO_ENABLED=0 go install filippo.io/age/cmd/age@${AGE_VERSION} \
    && CGO_ENABLED=0 go install filippo.io/age/cmd/age-keygen@${AGE_VERSION}

FROM ${POSTGRES_IMAGE}
RUN apk add --no-cache aws-cli coreutils jq minisign tzdata
COPY --from=age-builder /go/bin/age /usr/local/bin/age
COPY --from=age-builder /go/bin/age-keygen /usr/local/bin/age-keygen
COPY backup.sh /opt/nexaflow/backup
RUN chmod 0555 /opt/nexaflow/backup
USER 70:70
ENTRYPOINT ["/opt/nexaflow/backup"]
CMD ["help"]
