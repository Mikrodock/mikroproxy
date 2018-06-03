FROM busybox

COPY ./mikroproxy /home/

ENTRYPOINT [ "/home/mikroproxy" ] 