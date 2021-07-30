FROM golang:1.15.6

ENV APP_PATH=/app
RUN mkdir -p $APP_PATH
WORKDIR $APP_PATH

RUN groupadd -r appgroup \
  && useradd -r -g appgroup appuser \
  && chown -R appuser:appgroup $APP_PATH

COPY --chown=appuser:appgroup go.mod .
RUN  go mod download

COPY --chown=appuser:appgroup . .
RUN  go build -o app
RUN chown appuser:appgroup app

USER appuser

ENTRYPOINT ["./app"]
