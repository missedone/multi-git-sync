FROM public.ecr.aws/docker/library/alpine:3.21
# Copy binary defined in .goreleaser.yaml
COPY multi-git-sync /multi-git-sync

RUN apk add --no-cache tzdata git
ENV TZ=America/Los_Angeles

ENTRYPOINT ["/multi-git-sync"]
