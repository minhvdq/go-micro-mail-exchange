FROM alpine:latest
RUN mkdir /app
COPY complianceApp /app
CMD ["/app/complianceApp"]
