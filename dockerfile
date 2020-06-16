FROM gcr.io/distroless/static
MAINTAINER Clinton Reeder <clinton_reeder@byu.edu>

ARG NAME

COPY ${NAME} /via-control

ENTRYPOINT ["/via-control"]
