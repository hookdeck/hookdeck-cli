# Hookdeck CLI

The Hookdeck CLI helps you develop you webhook integrations on your local server.

**With the CLI, you can:**

- Securely test webhooks without relying on 3rd party software
- Trigger webhook events or resend events for easy testing
- Tail your API request logs in real-time
- Create, retrieve, update, or delete API objects.

![demo](docs/demo.gif)

## Installation

Hookdeck CLI is available for macOS, Windows, and Linux for distros like Ubuntu, Debian, RedHat and CentOS.

### macOS

Hookdeck CLI is available on macOS via [Homebrew](https://brew.sh/):

```sh
brew install hookdeck/hookdeck-cli/hookdeck
```

### Linux

Refer to the [installation instructions](https://hookdeck.com/docs/hookdeck-cli#install) for available Linux installation options.

### Windows

Hookdeck CLI is available on Windows via the [Scoop](https://scoop.sh/) package manager:

```sh
scoop bucket add hookdeck https://github.com/hookdeck/scoop-hookdeck-cli.git
scoop install hookdeck
```

### Docker

The CLI is also available as a Docker image: [`hookdeck/hookdeck-cli`](https://hub.docker.com/r/hookdeck/hookdeck-cli).

```sh
docker run --rm -it hookdeck/hookdeck-cli version
hookdeck version x.y.z (beta)
```

### Without package managers

Instructions are also available for installing and using the CLI [without a package manager](https://github.com/hookdeck/hookdeck-cli/wiki/Installing-and-updating#without-a-package-manager).

## Usage

Installing the CLI provides access to the `hookdeck` command.

```sh-session
hookdeck [command]

# Run `--help` for detailed information about CLI commands
hookdeck [command] help
```

## Commands

The Hookdeck CLI supports a broad range of commands. Below is some of the most used ones:
- [`login`](https://hookdeck.com/docs/cli/login)
- [`listen`](https://hookdeck.com/docs/cli/listen)

## Documentation

For a full reference, see the [CLI reference site](https://hookdeck.com/docs/cli)

## License
Copyright (c) Hookdeck. All rights reserved.

Licensed under the [Apache License 2.0 license](blob/master/LICENSE).

