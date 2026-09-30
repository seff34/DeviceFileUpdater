FROM alpine:3.20
RUN apk add --no-cache busybox-extras \
 && adduser -D -s /bin/sh dev && echo 'dev:devpass' | chpasswd \
 && mkdir -p /data && chown dev /data \
 && for t in base64 md5sum sha256sum od hexdump stat scp; do p=$(which $t) && rm -f "$p"; done; true
CMD ["telnetd", "-F", "-l", "/bin/login"]
