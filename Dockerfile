FROM alpine
RUN apk update && apk upgrade && \
  apk add --no-cache ca-certificates
COPY hookdeck /bin/hookdeck
ENTRYPOINT ["/bin/hookdeck"]
