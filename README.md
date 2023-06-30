# Hookdeck CLI

[slack-badge]: https://img.shields.io/badge/Slack-Hookdeck%20Developers-blue?logo=slack

[![slack-badge]](https://join.slack.com/t/hookdeckdevelopers/shared_invite/zt-yw7hlyzp-EQuO3QvdiBlH9Tz2KZg5MQ)

Using the Hookdeck CLI, you can forward your webhooks to your local webserver. We offer unlimited **free** and **permanent** webhook URLs. Your webhook history is preserved between session and can be viewed, replayed or used for testing by you and your teammates.

Hookdeck CLI is compatible with most of Hookdeck features such as filtering and fan-out delivery. You can use Hookdeck CLI to develop or test your webhook integration code locally.

Although it uses a different approach and philosophy, it's a replacement for ngrok and alternative HTTP tunnel solutions.

Hookdeck for development is completely free, and we monetize the platform with our Production offering.

For a complete reference, see the [CLI reference](https://hookdeck.com/cli)

![demo](docs/cli-demo.gif)

## Installation

Hookdeck CLI is available for macOS, Windows, and Linux for distros like Ubuntu, Debian, RedHat and CentOS.

### NPM

Hookdeck CLI is distributed as an NPM package:

```sh
npm install hookdeck -g
```

### macOS

Hookdeck CLI is available on macOS via [Homebrew](https://brew.sh/):

```sh
brew install hookdeck/hookdeck/hookdeck
```

### Windows

Hookdeck CLI is available on Windows via the [Scoop](https://scoop.sh/) package manager:

```sh
scoop bucket add hookdeck https://github.com/hookdeck/scoop-hookdeck-cli.git
scoop install hookdeck
```

### Linux Or Without package managers

To install the Hookdeck CLI on Linux without a package manager:

1. Download the latest linux tar.gz file from https://github.com/hookdeck/hookdeck-cli/releases/latest
2. Unzip the file: tar -xvf hookdeck_X.X.X_linux_amd64.tar.gz
3. Run the executable: ./hookdeck

### Docker

The CLI is also available as a Docker image: [`hookdeck/hookdeck-cli`](https://hub.docker.com/r/hookdeck/hookdeck-cli).

```sh
docker run --rm -it hookdeck/hookdeck-cli version
hookdeck version x.y.z (beta)
```

If you want to login to your Hookdeck account with the CLI and persist
credentials, you can bind mount the `~/.config/hookdeck` directory:

```sh
docker run --rm -it -v $HOME/.config/hookdeck:/root/.config/hookdeck hookdeck/hookdeck-cli login
```

Then you can listen on any of your sources. Don't forget to use
`host.docker.internal` to reach a port on your host machine, otherwise
that port will not be accessible from `localhost` inside the container.

```sh
docker run --rm -it -v $HOME/.config/hookdeck:/root/.config/hookdeck hookdeck/hookdeck-cli listen http://host.docker.internal:1234
```

## Usage

Installing the CLI provides access to the `hookdeck` command.

```sh-session
hookdeck [command]

# Run `--help` for detailed information about CLI commands
hookdeck [command] help
```

## Commands

### Login

Login with your Hookdeck account.

```sh-session
hookdeck login
```

> Login is optional, if you do not login a temporary guest account will be created for you when you run other commands.

### Listen

Start a session to forward your webhooks to an HTTP server.

```sh-session
hookdeck listen <port-or-URL> <source-alias?> <connection-query?>
```

Hookdeck works by routing webhooks receive for a given `source` (ie: Shopify, Github, etc.) to its defined `destination` by connecting them with a `connection` to a `destination`. The CLI allows you to receive webhooks for any given connection and forward them to your localhost at the specified port or any valid URL.

Each `source` is assigned a Webhook URL, which you can use to receive webhooks. When starting with a fresh account, the CLI will prompt you to create your first source. Each CLI process can listen to one source at a time.

Contrarily to ngrok, **Hookdeck does not allow to append a path to your Webhook URL**. Instead, the routing is done within Hookdeck configuration. This means you will also be prompted to specify your `destination` path, and you can have as many as you want per `source`.

> The `port-or-URL` param is mandatory, webhooks will be forwarded to http://localhost:$PORT/$DESTINATION_PATH when inputing a valid port or your provided URL.

#### Listen to all your connections for a given source

The second param, `source-alias` is used to select a specific source to listen on. By default, the CLI will start listening on all eligible connections for that source.

```sh-session
$ hookdeck listen 3000 shopify

👉  Inspect and replay webhooks: https://dashboard.hookdeck.com/cli/events

Shopify Source
🔌 Webhook URL: https://events.hookdeck.com/e/src_DAjaFWyyZXsFdZrTOKpuHnOH

Connections
Inventory Service forwarding to /webhooks/shopify/inventory
Orders Service forwarding to /webhooks/shopify/orders


⣾ Getting ready...

```

#### Listen to a subset of connection

The 3rd param, `connection-query` can be used to filter the list of connections the CLI will listen to. The connection query can either be the `connection` `alias` or the `path`

```sh-session
$ hookdeck listen 3000 shopify orders

👉  Inspect and replay webhooks: https://dashboard.hookdeck.com/cli/events

Shopify Source
🔌 Webhook URL: https://events.hookdeck.com/e/src_DAjaFWyyZXsFdZrTOKpuHnOH

Connections
Inventory Service forwarding to /webhooks/shopify/inventory


⣾ Getting ready...

```

#### Viewing and interacting with your webhooks

Webhooks logs for your CLI can be found at https://dashboard.hookdeck.com/cli/events. Events can be replayed or saved at any time.

### Logout

Logout of your Hookdeck account and clear your stored credentials.

```sh-session
hookdeck logout
```

### Skip SSL validation

If you are developing on an SSL destination, and are using a self-signed certificate, you can skip the SSL validation by using the flag `--insecure`.
You have to specify the full URL with the protocol when using this flag.

**This is dangerous, and should only be used in development scenarios, and for desitnations that you trust.**

```sh-session
hookdeck --insecure listen https://<url-or-url:port>/
```

### Version

Print your CLI version and whether or not a new version is available.

```sh-session
hookdeck version
```

### Completion

Configure auto-completion for Hookdeck CLI. It is run on install when using Homebrew or Scoop. You can optionally run this command when using the binaries directly or without a package manager.

```sh-session
hookdeck completion
```

### Running in CI

If you want to use Hookdeck in CI for tests or any other purposes, you can use your HOOKDECK_API_KEY to authenticate and start forwarding events.

```sh-session
$ hookdeck ci --api-key $HOOKDECK_API_KEY
Done! The Hookdeck CLI is configured in workspace MyWorkspace

$ hookdeck listen 3000 shopify orders

👉  Inspect and replay webhooks: https://dashboard.hookdeck.com/cli/events

Shopify Source
🔌 Webhook URL: https://events.hookdeck.com/e/src_DAjaFWyyZXsFdZrTOKpuHnOH

Connections
Inventory Service forwarding to /webhooks/shopify/inventory


⣾ Getting ready...

```

### Manage active workspace

If you are a part of many workspaces, you can switch between them using our workspace management commands.

```sh-session
$ hookdeck workspace list
My Workspace (current)
Another Workspace
Yet Another One

$ hookdeck workspace use
Use the arrow keys to navigate: ↓ ↑ → ←
? Select Workspace:
    My Workspace
    Another Workspace
  ▸ Yet Another One

Selecting workspace Yet Another One

$ hookdeck whoami
Using profile default
Logged in as Me in workspace Yet Another One
```

## Developing

Build from source by running:

```sh
go build
```

### Testing against a local API

When testing against a non-production Hookdeck API, you can use the
`--api-base` and `--ws-base` flags, e.g.:

```sh
./hookdeck-cli --api-base http://localhost:9000 --ws-base ws://localhost:3003 listen 1234
```

Also if running in Docker, the equivalent command would be:

```sh
docker run --rm -it \
    -v $HOME/.config/hookdeck:/root/.config/hookdeck hookdeck/hookdeck-cli \
    --api-base http://host.docker.internal:9000 \
    --ws-base ws://host.docker.internal:3003 \
    listen \
    http://host.docker.internal:1234
```

## License

Copyright (c) Hookdeck. All rights reserved.

Licensed under the [Apache License 2.0 license](blob/master/LICENSE).
