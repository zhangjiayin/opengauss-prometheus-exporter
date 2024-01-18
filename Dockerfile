FROM golang:1.15
WORKDIR /go/src/opengauss_exporter
COPY . .
#ENV GOPROXY=https://goproxy.cn GO111MODULE=on
ENV GOPROXY=https://goproxy.io,direct GO111MODULE=on
RUN make build

# Distribution
FROM debian:10-slim
COPY --from=0 /go/src/opengauss_exporter/bin/opengauss_exporter /bin/opengauss_exporter
COPY og_exporter_default.yaml  /etc/og_exporter/
COPY scripts/docker-entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/docker-entrypoint.sh; ln -s /usr/local/bin/docker-entrypoint.sh / # backwards compat

ENTRYPOINT ["docker-entrypoint.sh"]
EXPOSE 9187
CMD [ "opengauss_exporter" ]
