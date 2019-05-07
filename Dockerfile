FROM progrium/busybox

RUN mkdir -p /app
WORKDIR /app

ADD pod-broadcaster /app/pod-broadcaster

EXPOSE 8080

CMD [""]
ENTRYPOINT ["/app/pod-broadcaster"]
