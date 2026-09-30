FROM alpine:3.20
RUN apk add --no-cache dropbear && mkdir -p /etc/dropbear \
 && adduser -D -s /bin/sh dev && echo 'dev:devpass' | chpasswd \
 && mkdir -p /data && chown dev /data
CMD ["dropbear", "-F", "-E", "-R", "-p", "22"]
