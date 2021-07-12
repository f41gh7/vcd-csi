FROM amd64/alpine:3.10
RUN apk add --no-cache ca-certificates \
                       e2fsprogs \
                       findmnt \
                       xfsprogs \
                       blkid \
                       e2fsprogs-extra

ADD vcd-csi /bin/

ENTRYPOINT ["/bin/vcd-csi"]