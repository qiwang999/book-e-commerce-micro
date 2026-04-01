FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata curl \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime

COPY bin/linux/ /app/
WORKDIR /app
