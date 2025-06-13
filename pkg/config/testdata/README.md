# Config testdata

Some explanation of different config testdata scenarios:

- default-profile.toml: This config has a singular profile named "default".
- empty.toml: This config is completely empty.
- local-full.toml: This config is for local config `${PWD}/.hookdeck/config.toml` where the user has a full profile.
- local-workspace-only.toml: This config is for local config `${PWD}/.hookdeck/config.toml` where the user only has a `workspace_id` config. This happens when user runs `$ hookdeck project use --local` to scope the usage of the project within their local scope.
