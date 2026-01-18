FROM golang:1.25 AS builder

WORKDIR /app
COPY . .

RUN make &&\
    wget https://github.com/v2fly/domain-list-community/releases/latest/download/dlc.dat -O build/geosite.dat &&\
    wget https://github.com/v2fly/geoip/releases/latest/download/geoip.dat -O build/geoip.dat &&\
    wget https://github.com/v2fly/geoip/releases/latest/download/geoip-only-cn-private.dat -O build/geoip-only-cn-private.dat

FROM alpine
WORKDIR /
RUN apk add --no-cache tzdata ca-certificates
COPY --from=builder /trojan-go/build /usr/local/bin/
COPY --from=builder /trojan-go/example/server.json /etc/trojan-go/config.json

ENTRYPOINT ["/usr/local/bin/trojan-go", "-config"]
CMD ["/etc/trojan-go/config.json"]
