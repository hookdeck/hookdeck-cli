# Hookdeck CLI

[slack-badge]: https://img.shields.io/badge/Slack-Hookdeck%20Developers-blue?logo=slack

[![slack-badge]](https://join.slack.com/t/hookdeckdevelopers/shared_invite/zt-yw7hlyzp-EQuO3QvdiBlH9Tz2KZg5MQ)

Using the Hookdeck CLI, you can forward your events (e.g. webhooks) to your local web server with unlimited **free** and **permanent** event URLs. Your event history is preserved between sessions and can be viewed, replayed, or used for testing by you and your teammates.

Hookdeck CLI is compatible with most of Hookdeck's features, such as filtering and fan-out delivery. You can use Hookdeck CLI to develop or test your event (e.g. webhook) integration code locally.

Although it uses a different approach and philosophy, it's a replacement for ngrok and alternative HTTP tunnel solutions.

Hookdeck for development is completely free, and we monetize the platform with our production offering.

For a complete reference, see the [CLI reference](https://hookdeck.com/docs/cli?ref=github-hookdeck-cli).

https://github.com/user-attachments/assets/5fca7842-9c41-411c-8cd6-2f32f84fa907

## Installation

Hookdeck CLI is available for macOS, Windows, and Linux for distros like Ubuntu, Debian, RedHat, and CentOS.

### NPM

Hookdeck CLI is distributed as an NPM package:

```sh
npm install hookdeck-cli -g
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

### Linux Or without package managers

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

```sh
hookdeck [command]

# Run `--help` for detailed information about CLI commands
hookdeck [command] help
```

## Commands

### Login

Login with your Hookdeck account. This will typically open a browser window for authentication.

```sh
hookdeck login
```

If you are in an environment without a browser (e.g., a TTY-only terminal), you can use the `--interactive` (or `-i`) flag to log in by pasting your API key:
```sh
hookdeck login --interactive
```

> Login is optional, if you do not login a temporary guest account will be created for you when you run other commands.

### Listen

Start a session to forward your events to an HTTP server.

```sh
hookdeck listen <port-or-URL> <source-alias?> <connection-query?> [--path?]
```

Hookdeck works by routing events received for a given `source` (i.e., Shopify, Github, etc.) to its defined `destination` by connecting them with a `connection` to a `destination`. The CLI allows you to receive events for any given connection and forward them to your localhost at the specified port or any valid URL.

Each `source` is assigned an Event URL, which you can use to receive events. When starting with a fresh account, the CLI will prompt you to create your first source. Each CLI process can listen to one source at a time.

Contrary to ngrok, **Hookdeck does not allow to append a path to your event URL**. Instead, the routing is done within Hookdeck configuration. This means you will also be prompted to specify your `destination` path, and you can have as many as you want per `source`.

> The `port-or-URL` param is mandatory, events will be forwarded to http://localhost:$PORT/$DESTINATION_PATH when inputing a valid port or your provided URL.

#### Listen to all your connections for a given source

The second param, `source-alias` is used to select a specific source to listen on. By default, the CLI will start listening on all eligible connections for that source.

```sh
$ hookdeck listen 3000 shopify

👉  Inspect and replay events: https://dashboard.hookdeck.com/cli/events

Shopify Source
🔌 Event URL: https://events.hookdeck.com/e/src_DAjaFWyyZXsFdZrTOKpuHnOH

Connections
Inventory Service forwarding to /webhooks/shopify/inventory
Orders Service forwarding to /webhooks/shopify/orders


⣾ Getting ready...

```

#### Listen to multiple sources

`source-alias` can be a comma-separated list of source names (for example, `stripe,shopify,twilio`) or `'*'` (with quotes) to listen to all sources.

```sh
$ hookdeck listen 3000 '*'

👉  Inspect and replay events: https://dashboard.hookdeck.com/cli/events

Sources
🔌 stripe URL: https://events.hookdeck.com/e/src_DAjaFWyyZXsFdZrTOKpuHn01
🔌 shopify URL: https://events.hookdeck.com/e/src_DAjaFWyyZXsFdZrTOKpuHn02
🔌 twilio URL: https://events.hookdeck.com/e/src_DAjaFWyyZXsFdZrTOKpuHn03

Connections
stripe -> cli-stripe forwarding to /webhooks/stripe
shopify -> cli-shopify forwarding to /webhooks/shopify
twilio -> cli-twilio forwarding to /webhooks/twilio

⣾ Getting ready...

```

#### Listen to a subset of connections

The 3rd param, `connection-query` specifies which connection with a CLI destination to adopt for listening. By default, the first connection with a CLI destination type will be used. If a connection with the specified name doesn't exist, a new connection will be created with the passed value. The connection query is checked against the `connection` name, `alias`, and the `path` values.

```sh
$ hookdeck listen 3000 shopify orders

👉  Inspect and replay events: https://dashboard.hookdeck.com/cli/events

Shopify Source
🔌 Event URL: https://events.hookdeck.com/e/src_DAjaFWyyZXsFdZrTOKpuHnOH

Connections
Orders Service forwarding to /webhooks/shopify/orders


⣾ Getting ready...

```

#### Changing the path events are forwarded to

The `--path` flag sets the path to which events are forwarded.

```sh
$ hookdeck listen 3000 shopify orders --path /events/shopify/orders

👉  Inspect and replay events: https://dashboard.hookdeck.com/cli/events

Shopify Source
🔌 Event URL: https://events.hookdeck.com/e/src_DAjaFWyyZXsFdZrTOKpuHnOH

Connections
Orders Service forwarding to /events/shopify/orders


⣾ Getting ready...

```

#### Viewing and interacting with your events

Event logs for your CLI can be found at [https://dashboard.hookdeck.com/cli/events](https://dashboard.hookdeck.com/cli/events?ref=github-hookdeck-cli). Events can be replayed or saved at any time.

### Logout

Logout of your Hookdeck account and clear your stored credentials.

```sh
hookdeck logout
```

### Skip SSL validation

When forwarding events to an HTTPS URL as the first argument to `hookdeck listen` (e.g., `https://localhost:1234/webhook`), you might encounter SSL validation errors if the destination is using a self-signed certificate.

For local development scenarios, you can instruct the `listen` command to bypass this SSL certificate validation by using its `--insecure` flag. You must provide the full HTTPS URL.

**This is dangerous and should only be used in trusted local development environments for destinations you control.**

Example of skipping SSL validation for an HTTPS destination:
```sh
hookdeck listen --insecure https://<your-ssl-url-or-url:port>/ <source-alias?> <connection-query?>
```

### Version

Print your CLI version and whether or not a new version is available.

```sh
hookdeck version
```

### Completion

Configure auto-completion for Hookdeck CLI. It is run on install when using Homebrew or Scoop. You can optionally run this command when using the binaries directly or without a package manager.

```sh
hookdeck completion
```

### Running in CI

If you want to use Hookdeck in CI for tests or any other purposes, you can use your HOOKDECK_API_KEY to authenticate and start forwarding events.

```sh
$ hookdeck ci --api-key $HOOKDECK_API_KEY
Done! The Hookdeck CLI is configured in project MyProject

$ hookdeck listen 3000 shopify orders

👉  Inspect and replay events: https://dashboard.hookdeck.com/cli/events

Shopify Source
🔌 Event URL: https://events.hookdeck.com/e/src_DAjaFWyyZXsFdZrTOKpuHnOH

Connections
Inventory Service forwarding to /webhooks/shopify/inventory


⣾ Getting ready...

```

### Manage active project

If you are a part of multiple projects, you can switch between them using our project management commands.

To list your projects, you can use the `hookdeck project list` command. It can take optional organization and project name substrings to filter the list. The matching is partial and case-insensitive.

```sh
# List all projects
$ hookdeck project list
My Org / My Project (current)
My Org / Another Project
Another Org / Yet Another One

# List projects with "Org" in the organization name and "Proj" in the project name
$ hookdeck project list Org Proj
My Org / My Project (current)
My Org / Another Project
```

To select or change the active project, use the `hookdeck project use` command. When arguments are provided, it uses exact, case-insensitive matching for the organization and project names.

```console
hookdeck project use [<organization_name> [<project_name>]]
```

**Behavior:**

-   **`hookdeck project use`** (no arguments):
    An interactive prompt will guide you through selecting your organization and then the project within that organization.
    ```sh
    $ hookdeck project use
    Use the arrow keys to navigate: ↓ ↑ → ←
    ? Select Organization:
        My Org
      ▸ Another Org
    ...
    ? Select Project (Another Org):
        Project X
      ▸ Project Y
    Selecting project Project Y
    Successfully set active project to: [Another Org] Project Y
    ```

-   **`hookdeck project use <organization_name>`** (one argument):
    Filters projects by the specified `<organization_name>`.
    - If multiple projects exist under that organization, you'll be prompted to choose one.
    - If only one project exists, it will be selected automatically.
    ```sh
    $ hookdeck project use "My Org"
    # (If multiple projects, prompts to select. If one, auto-selects)
    Successfully set active project to: [My Org] Default Project
    ```

-   **`hookdeck project use <organization_name> <project_name>`** (two arguments):
    Directly selects the project `<project_name>` under the organization `<organization_name>`.
    ```sh
    $ hookdeck project use "My Corp" "API Staging"
    Successfully set active project to: [My Corp] API Staging
    ```

Upon successful selection, you will generally see a confirmation message like:
`Successfully set active project to: [<organization_name>] <project_name>`

## Configuration files

The Hookdeck CLI uses configuration files to store the your keys, project settings, profiles, and other configurations.

### Configuration file name and locations

The CLI will look for the configuration file in the following order:

  1. The `--config` flag, which allows you to specify a custom configuration file name and path per command.
  2. The local directory `.hookdeck/config.toml`.
  3. The default global configuration file location.

### Default configuration Location

The default configuration location varies by operating system:

- **macOS/Linux**: `~/.config/hookdeck/config.toml`
- **Windows**: `%USERPROFILE%\.config\hookdeck\config.toml`

The CLI follows the [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html) on Unix-like systems, respecting the `XDG_CONFIG_HOME` environment variable if set.

### Configuration File Format

The Hookdeck CLI configuration file is stored in TOML format and typically includes:

```toml
api_key = "api_key_xxxxxxxxxxxxxxxxxxxx"
project_id = "tm_xxxxxxxxxxxxxxx"
project_mode = "inbound" | "console"
```

### Local Configuration

The Hookdeck CLI also supports local configuration files. If you run the CLI commands in a directory that contains a `.hookdeck/config.toml` file, the CLI will use that file for configuration instead of the global one.

### Using Profiles

The `config.toml` file supports profiles which give you the ability to save different CLI configuration within the same configuration file.

You can create new profiles by either running `hookdeck login` or `hookdeck use` with the `-p` flag and a profile name. For example:

```sh
hookdeck login -p dev
```

If you know the name of your Hookdeck organization and the project you want to use with a profile you can use the following:

```sh
hookdeck project use org_name proj_name -p prod
```

This will results in the following config file that has two profiles:

```toml
profile = "dev"

[dev]
  api_key = "api_key_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
  project_id = "tm_5JxTelcYxOJy"
  project_mode = "inbound"

[prod]
  api_key = "api_key_yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy"
  project_id = "tm_U9Zod13qtsHp"
  project_mode = "inbound"
```

This allows you to run commands against different projects. For example, to listen to the `webhooks` source in the `dev` profile, run:

```sh
hookdeck listen 3030 webhooks -p dev
```

To listen to the `webhooks` source in the `prod` profile, run:

```sh
hookdeck listen 3030 webhooks -p prod
```

## Global Flags

The following flags can be used with any command:

*   `--api-key`: Your API key to use for the command.
*   `--color`: Turn on/off color output (on, off, auto).
*   `--config`: Path to a specific configuration file.
*   `--device-name`: A unique name for your device.
*   `--insecure`: Allow invalid TLS certificates.
*   `--log-level`: Set the logging level (debug, info, warn, error).
*   `--profile` or `-p`: Use a specific configuration profile.

There are also some hidden flags that are mainly used for development and debugging:

*   `--api-base`: Sets the API base URL.
*   `--dashboard-base`: Sets the web dashboard base URL.
*   `--console-base`: Sets the web console base URL.
*   `--ws-base`: Sets the Websocket base URL.


## Developing

Running from source:

```sh
go run main.go
```

Build from source by running:

```sh
go build
```

Then run the locally generated `hookdeck-cli` binary:

```sh
./hookdeck-cli
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

Licensed under the [Apache License 2.0 license](LICENSE).
