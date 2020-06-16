FROM gcr.io/distroless/static
MAINTAINER Clinton Reeder <clinton_reeder@byu.edu>

COPY via-control /via-control

ENTRYPOINT ["/via-control"]
