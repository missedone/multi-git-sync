Multi-Git-Sync
===

## Overview

`multi-git-sync` is a cli to sync the (multi)-git repo(s) with crontab-like scheduler, it can be used as the replacement of https://github.com/kubernetes/git-sync

## Features

* [x] crontab-like scheduler
* [x] git full clone
* [x] git shallow clone
* [x] git sparse checkout

## Usage

### Run CLI
* show help info
  ```shell
  ./multi-git-sync -h
  ```
* run with sparse-checkout (aka, partial clone)
  ```shell
  ./multi-git-sync -config examples/sparse-checkout/config.yaml
  ```
* run with full-clone
  ```shell
  ./multi-git-sync -config examples/clone/config.yaml
  ```
* run with shallow and partial clone
  ```shell
  ./multi-git-sync -config examples/shallow/config.yaml
  ```

### Run Docker
```shell
docker run --rm -v $(pwd)/examples/sparse-checkout:/opt/git-sync/ multi-git-sync:main -config=/opt/git-sync/config.yaml
```

## Development

### Setup Local Dev

```shell
brew install go
brew install goreleaser
brew install golangci-lint
curl -fsSL https://github.com/gotestyourself/gotestsum/releases/download/v1.12.0/gotestsum_1.12.0_darwin_arm64.tar.gz | tar -xz -C ~/bin
```

### Build and test

```shell
make build
```