FROM alpine:3.20
RUN apk add --no-cache busybox-extras \
 && adduser -D -s /bin/sh dev && echo 'dev:devpass' | chpasswd \
 && mkdir -p /data && chown dev /data
# busybox-extras ftpd has no FTP authentication: -A skips login, tcpsvd -u runs it as dev.
CMD ["sh", "-c", "telnetd -l /bin/login && exec tcpsvd -vE -u dev 0.0.0.0 21 ftpd -A -w /"]
