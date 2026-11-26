# The nightly backup job (render.yaml: yetibyrash-backup). Postgres 16 tools to match the
# database, rclone to store the file in R2.
FROM alpine:3.20
RUN apk add --no-cache postgresql16-client rclone ca-certificates tzdata
COPY deploy/backup/backup.sh deploy/backup/restore.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/backup.sh /usr/local/bin/restore.sh
CMD ["/usr/local/bin/backup.sh"]
