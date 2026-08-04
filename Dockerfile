FROM alpine
RUN apk update && apk upgrade && \
  apk add --no-cache ca-certificates
ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/hookdeck /bin/hookdeck
ENTRYPOINT ["/bin/hookdeck"]
