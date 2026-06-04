# Installation

Install these tools before running the project:

- Docker with Docker Compose
- Go 1.22 or newer
- Task
- curl
- jq
- yq

## macOS

Using Homebrew:

```bash
brew install go-task/tap/go-task go jq yq
```

Install Docker Desktop separately:

```txt
https://www.docker.com/products/docker-desktop/
```

## Ubuntu

Using apt and the official Docker convenience script:

```bash
sudo apt-get update
sudo apt-get install -y curl jq yq golang-go
sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b ~/.local/bin
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker "$USER"
```

After adding your user to the `docker` group, log out and log back in before running Docker commands.

## Verify

```bash
docker --version
docker compose version
task --version
go version
jq --version
yq --version
curl --version
```
