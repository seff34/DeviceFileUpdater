FROM alpine:3.20
RUN apk add --no-cache openssh openssh-sftp-server && ssh-keygen -A \
 && adduser -D -s /bin/sh dev && echo 'dev:devpass' | chpasswd \
 && mkdir -p /data && chown dev /data \
 && sed -i 's/^#\?PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config
CMD ["/usr/sbin/sshd", "-D", "-e"]
