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

To install a beta (pre-release) version:

```sh
npm install hookdeck-cli@beta -g
```

### macOS

Hookdeck CLI is available on macOS via [Homebrew](https://brew.sh/):

```sh
brew install hookdeck/hookdeck/hookdeck
```

To install a beta (pre-release) version:

```sh
brew install hookdeck/hookdeck/hookdeck-beta
```

### Windows

Hookdeck CLI is available on Windows via the [Scoop](https://scoop.sh/) package manager:

```sh
scoop bucket add hookdeck https://github.com/hookdeck/scoop-hookdeck-cli.git
scoop install hookdeck
```

To install a beta (pre-release) version:

```sh
scoop install hookdeck/scoop-hookdeck/hookdeck-beta
```

### Linux Or without package managers

To install the Hookdeck CLI on Linux without a package manager:

1. Download the latest linux tar.gz file from https://github.com/hookdeck/hookdeck-cli/releases/latest
2. Unzip the file: tar -xvf hookdeck_X.X.X_linux_amd64.tar.gz
3. Run the executable: ./hookdeck

For beta (pre-release) versions, download the `.deb` or `.rpm` packages from the [GitHub releases page](https://github.com/hookdeck/hookdeck-cli/releases) (look for releases marked as "Pre-release").

### Docker

The CLI is also available as a Docker image: [`hookdeck/hookdeck-cli`](https://hub.docker.com/r/hookdeck/hookdeck-cli).

```sh
docker run --rm -it hookdeck/hookdeck-cli version
hookdeck version x.y.z (beta)
```

To use a specific version (including beta releases), specify the version tag:

```sh
docker run --rm -it hookdeck/hookdeck-cli:v1.2.3-beta.1 version
```

Note: Beta releases do not update the `latest` tag. Only stable releases update `latest`.

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
hookdeck listen <port-or-URL> <source-alias?> <connection-query?> [--path?] [--output?]
```

Hookdeck works by routing events received for a given `source` (i.e., Shopify, Github, etc.) to its defined `destination` by connecting them with a `connection` to a `destination`. The CLI allows you to receive events for any given connection and forward them to your localhost at the specified port or any valid URL.

Each `source` is assigned an Event URL, which you can use to receive events. When starting with a fresh account, the CLI will prompt you to create your first source. Each CLI process can listen to one source at a time.

> The `port-or-URL` param is mandatory, events will be forwarded to http://localhost:$PORT/$DESTINATION_PATH when inputing a valid port or your provided URL.

#### Interactive Mode

The default interactive mode uses a full-screen TUI (Terminal User Interface) with an alternative screen buffer, meaning your terminal history is preserved when you exit. The interface includes:

- **Connection Header**: Shows your sources, webhook URLs, and connection routing
  - Auto-collapses when the first event arrives to save space
  - Toggle with `i` to expand/collapse connection details
- **Event List**: Scrollable history of all received events (up to 1000 events)
  - Auto-scrolls to show latest events as they arrive
  - Manual navigation pauses auto-scrolling
- **Status Bar**: Shows event details and available keyboard shortcuts
- **Event Details View**: Full request/response inspection with headers and body

#### Interactive Keyboard Shortcuts

While in interactive mode, you can use the following keyboard shortcuts:

- `↑` / `↓` or `k` / `j` - Navigate between events (select different events)
- `i` - Toggle connection information (expand/collapse connection details)
- `r` - Retry the selected event
- `o` - Open the selected event in the Hookdeck dashboard
- `d` - Show detailed request/response information for the selected event (press `d` or `ESC` to close)
  - When details view is open: `↑` / `↓` scroll through content, `PgUp` / `PgDown` for page navigation
- `q` - Quit the application (terminal state is restored)
- `Ctrl+C` - Also quits the application

The selected event is indicated by a `>` character at the beginning of the line. All actions (retry, open, details) work on the currently selected event, not just the latest one. These shortcuts are displayed in the status bar at the bottom of the screen.

#### Listen to all your connections for a given source

The second param, `source-alias` is used to select a specific source to listen on. By default, the CLI will start listening on all eligible connections for that source.

```sh
$ hookdeck listen 3000 shopify

●── HOOKDECK CLI ──●

Listening on 1 source • 2 connections • [i] Collapse

Shopify Source
│  Requests to → https://events.hookdeck.com/e/src_DAjaFWyyZXsFdZrTOKpuHnOH
├─ Forwards to → http://localhost:3000/webhooks/shopify/inventory (Inventory Service)
└─ Forwards to → http://localhost:3000/webhooks/shopify/orders (Orders Service)

💡 Open dashboard to inspect, retry & bookmark events: https://dashboard.hookdeck.com/events/cli?team_id=...

Events • [↑↓] Navigate ──────────────────────────────────────────────────────────

2025-10-12 14:32:15 [200] POST http://localhost:3000/webhooks/shopify/orders (23ms) → https://dashboard.hookdeck.com/events/evt_...
> 2025-10-12 14:32:18 [200] POST http://localhost:3000/webhooks/shopify/inventory (45ms) → https://dashboard.hookdeck.com/events/evt_...

───────────────────────────────────────────────────────────────────────────────
> ✓ Last event succeeded with status 200 | [r] Retry • [o] Open in dashboard • [d] Show data
```

#### Listen to multiple sources

`source-alias` can be a comma-separated list of source names (for example, `stripe,shopify,twilio`) or `'*'` (with quotes) to listen to all sources.

```sh
$ hookdeck listen 3000 '*'

●── HOOKDECK CLI ──●

Listening on 3 sources • 3 connections • [i] Collapse

stripe
│  Requests to → https://events.hookdeck.com/e/src_DAjaFWyyZXsFdZrTOKpuHn01
└─ Forwards to → http://localhost:3000/webhooks/stripe (cli-stripe)

shopify
│  Requests to → https://events.hookdeck.com/e/src_DAjaFWyyZXsFdZrTOKpuHn02
└─ Forwards to → http://localhost:3000/webhooks/shopify (cli-shopify)

twilio
│  Requests to → https://events.hookdeck.com/e/src_DAjaFWyyZXsFdZrTOKpuHn03
└─ Forwards to → http://localhost:3000/webhooks/twilio (cli-twilio)

💡 Open dashboard to inspect, retry & bookmark events: https://dashboard.hookdeck.com/events/cli?team_id=...

Events • [↑↓] Navigate ──────────────────────────────────────────────────────────

2025-10-12 14:35:21 [200] POST http://localhost:3000/webhooks/stripe (12ms) → https://dashboard.hookdeck.com/events/evt_...
2025-10-12 14:35:44 [200] POST http://localhost:3000/webhooks/shopify (31ms) → https://dashboard.hookdeck.com/events/evt_...
> 2025-10-12 14:35:52 [200] POST http://localhost:3000/webhooks/twilio (18ms) → https://dashboard.hookdeck.com/events/evt_...

───────────────────────────────────────────────────────────────────────────────
> ✓ Last event succeeded with status 200 | [r] Retry • [o] Open in dashboard • [d] Show data
```

#### Listen to a subset of connections

The 3rd param, `connection-query` specifies which connection with a CLI destination to adopt for listening. By default, the first connection with a CLI destination type will be used. If a connection with the specified name doesn't exist, a new connection will be created with the passed value. The connection query is checked against the `connection` name, `alias`, and the `path` values.

```sh
$ hookdeck listen 3000 shopify orders

●── HOOKDECK CLI ──●

Listening on 1 source • 1 connection • [i] Collapse

Shopify Source
│  Requests to → https://events.hookdeck.com/e/src_DAjaFWyyZXsFdZrTOKpuHnOH
└─ Forwards to → http://localhost:3000/webhooks/shopify/orders (Orders Service)

💡 Open dashboard to inspect, retry & bookmark events: https://dashboard.hookdeck.com/events/cli?team_id=...

Events • [↑↓] Navigate ──────────────────────────────────────────────────────────

> 2025-10-12 14:38:09 [200] POST http://localhost:3000/webhooks/shopify/orders (27ms) → https://dashboard.hookdeck.com/events/evt_...

───────────────────────────────────────────────────────────────────────────────
> ✓ Last event succeeded with status 200 | [r] Retry • [o] Open in dashboard • [d] Show data
```

#### Changing the path events are forwarded to

The `--path` flag sets the path to which events are forwarded.

```sh
$ hookdeck listen 3000 shopify orders --path /events/shopify/orders

●── HOOKDECK CLI ──●

Listening on 1 source • 1 connection • [i] Collapse

Shopify Source
│  Requests to → https://events.hookdeck.com/e/src_DAjaFWyyZXsFdZrTOKpuHnOH
└─ Forwards to → http://localhost:3000/events/shopify/orders (Orders Service)

💡 Open dashboard to inspect, retry & bookmark events: https://dashboard.hookdeck.com/events/cli?team_id=...

Events • [↑↓] Navigate ──────────────────────────────────────────────────────────

> 2025-10-12 14:40:23 [200] POST http://localhost:3000/events/shopify/orders (19ms) → https://dashboard.hookdeck.com/events/evt_...

───────────────────────────────────────────────────────────────────────────────
> ✓ Last event succeeded with status 200 | [r] Retry • [o] Open in dashboard • [d] Show data
```

#### Controlling output verbosity

The `--output` flag controls how events are displayed. This is useful for reducing resource usage in high-throughput scenarios or when running in the background.

**Available modes:**

- `interactive` (default) - Full-screen TUI with alternative screen buffer, event history, navigation, and keyboard shortcuts. Your terminal history is preserved and restored when you exit.
- `compact` - Simple one-line logs for all events without interactive features. Events are appended to your terminal history.
- `quiet` - Only displays fatal connection errors (network failures, timeouts), not HTTP errors

All modes display connection information at startup and a connection status message.

**Examples:**

```sh
# Default - full interactive UI with keyboard shortcuts
$ hookdeck listen 3000 shopify

# Simple logging mode - prints all events as one-line logs
$ hookdeck listen 3000 shopify --output compact

# Quiet mode - only shows fatal connection errors
$ hookdeck listen 3000 shopify --output quiet
```

**Compact mode output:**

```
Listening on
shopify
└─ Forwards to → http://localhost:3000

Connected. Waiting for events...

2025-10-08 15:56:53 [200] POST http://localhost:3000 (45ms) → https://...
2025-10-08 15:56:54 [422] POST http://localhost:3000 (12ms) → https://...
```

**Quiet mode output:**

```
Listening on
shopify
└─ Forwards to → http://localhost:3000

Connected. Waiting for events...

2025-10-08 15:56:53 [ERROR] Failed to POST: connection refused
```

> Note: In `quiet` mode, only fatal errors are shown (connection failures, network unreachable, timeouts). HTTP error responses (4xx, 5xx) are not displayed as they are valid HTTP responses.

#### Filtering events

The CLI supports filtering events using Hookdeck's filter syntax. Filters allow you to receive only events that match specific conditions, reducing noise and focusing on the events you care about during development.

**Filter flags:**

- `--filter-body` - Filter events by request body content (JSON)
- `--filter-headers` - Filter events by request headers (JSON)
- `--filter-query` - Filter events by query parameters (JSON)
- `--filter-path` - Filter events by request path (JSON)

All filter flags accept JSON using [Hookdeck's filter syntax](https://hookdeck.com/docs/filters). You can use exact matches or operators like `$exist`, `$gte`, `$lte`, `$in`, etc.

**Examples:**

```sh
# Filter events by body content (only events with matching data)
hookdeck listen 3000 github --filter-body '{"action": "opened"}'

# Filter events with multiple conditions
hookdeck listen 3000 stripe --filter-body '{"type": "charge.succeeded"}' --filter-headers '{"x-stripe-signature": {"$exist": true}}'

# Filter using operators
hookdeck listen 3000 api --filter-body '{"amount": {"$gte": 100}}'
```

When filters are active, the CLI will display a warning message indicating which filters are applied. Only events matching all specified filter conditions will be forwarded to your local server.

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

●── HOOKDECK CLI ──●

Listening on 1 source • 1 connection • [i] Collapse

Shopify Source
│  Requests to → https://events.hookdeck.com/e/src_DAjaFWyyZXsFdZrTOKpuHnOH
└─ Forwards to → http://localhost:3000/webhooks/shopify/orders (Orders Service)

💡 Open dashboard to inspect, retry & bookmark events: https://dashboard.hookdeck.com/events/cli?team_id=...

Events • [↑↓] Navigate ──────────────────────────────────────────────────────────

> 2025-10-12 14:42:55 [200] POST http://localhost:3000/webhooks/shopify/orders (34ms) → https://dashboard.hookdeck.com/events/evt_...

───────────────────────────────────────────────────────────────────────────────
> ✓ Last event succeeded with status 200 | [r] Retry • [o] Open in dashboard • [d] Show data
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

- **`hookdeck project use`** (no arguments):
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

- **`hookdeck project use <organization_name>`** (one argument):
  Filters projects by the specified `<organization_name>`.

  - If multiple projects exist under that organization, you'll be prompted to choose one.
  - If only one project exists, it will be selected automatically.

  ```sh
  $ hookdeck project use "My Org"
  # (If multiple projects, prompts to select. If one, auto-selects)
  Successfully set active project to: [My Org] Default Project
  ```

- **`hookdeck project use <organization_name> <project_name>`** (two arguments):
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

- `--api-key`: Your API key to use for the command.
- `--color`: Turn on/off color output (on, off, auto).
- `--config`: Path to a specific configuration file.
- `--device-name`: A unique name for your device.
- `--insecure`: Allow invalid TLS certificates.
- `--log-level`: Set the logging level (debug, info, warn, error).
- `--profile` or `-p`: Use a specific configuration profile.

There are also some hidden flags that are mainly used for development and debugging:

*   `--api-base`: Sets the API base URL.
*   `--dashboard-base`: Sets the web dashboard base URL.
*   `--console-base`: Sets the web console base URL.
*   `--ws-base`: Sets the Websocket base URL.

## Troubleshooting

### Homebrew: Binary Already Exists Error

If you previously installed Hookdeck via the Homebrew formula and are upgrading to the cask version, you may see:

```
Warning: It seems there is already a Binary at '/opt/homebrew/bin/hookdeck'
from formula hookdeck; skipping link.
```

To resolve this, uninstall the old formula version first, then install the cask:

```sh
brew uninstall hookdeck
brew install --cask hookdeck/hookdeck/hookdeck
```


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
