# Dependencies are installed while preparing the observer; the acceptance run
# itself has --network none and mounts only a prebuilt binary and observer.
FROM python:3.12-alpine
RUN apk add --no-cache strace
