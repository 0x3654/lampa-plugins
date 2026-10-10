# marks — «одна база» отметок Lampa (build + тесты, финал — только бинарник)
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go main_test.go ./
ARG GIT_REV=unknown
RUN go test ./... && \
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.rev=${GIT_REV}" -o /marks .

FROM alpine:3.21
RUN apk add --no-cache tini ca-certificates && \
    adduser -D -u 1000 marks
COPY --from=build /marks /usr/local/bin/marks
USER 1000
VOLUME /data
EXPOSE 8356
HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://127.0.0.1:8356/healthz || exit 1
ENTRYPOINT ["/sbin/tini", "--", "marks"]
CMD ["-addr", ":8356", "-data", "/data"]

LABEL org.opencontainers.image.title="marks" \
      org.opencontainers.image.description="Stable single store for Lampa watch marks" \
      org.opencontainers.image.source="https://github.com/0x3654/lampa-plugins" \
      org.opencontainers.image.base.name="docker.io/library/alpine:3.21"
