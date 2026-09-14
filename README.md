# ⚡ Volok

Volok is a minimal personal VLESS node library. It stores direct VLESS share
links in a single JSON file, serves them as a subscription to reader tokens,
and ships a VPS installer that sets up Xray and registers the node back into
the library.

There is no web UI, no database, no user management and no relay/proxy of
your traffic: Volok only manages and distributes direct links to your own
servers.

## How it works

```text
Manage:    volok CLI on the router -> volok.json
Install:   SSH into a VPS, run the installer from Volok
Subscribe: VPN client pulls /sub with a user token
Traffic:   client -> Xray on the VPS -> internet (direct, no relay)
```

## Build

```sh
go build -trimpath -ldflags="-s -w" -o volok ./cmd/volok
```

Cross builds for Linux amd64/arm64/armv7 are supported (no CGO).

## Quick start on the router

```sh
# Create the library (refuses to overwrite an existing file)
volok --file /etc/volok/volok.json init --public-url https://vpn.example.com

# Start the HTTP server
volok --file /etc/volok/volok.json serve

# Or install it as a service
volok --file /etc/volok/volok.json service install
volok --file /etc/volok/volok.json service start
```

`volok.json` holds the admin token (first field), a list of `users` tokens,
the listen address, the public URL and the node library. Keep the file
protected: it contains credentials.

## Adding a node manually

```sh
volok --file /etc/volok/volok.json node add --name "Finland" --url "vless://…"
volok --file /etc/volok/volok.json node list
```

## Adding a new VPS automatically

After logging into the VPS over SSH, run the installer command:

```sh
volok --file /etc/volok/volok.json install-command
# e.g. curl -fsS --proto '=https' 'https://vpn.example.com/register?token=<ADMIN>' | bash
```

The installer:

- detects the OS/init/architecture and installs a pinned Xray release with
  checksum verification;
- generates a VLESS + REALITY + Vision identity **once** (re-running the
  installer keeps the same UUID, keys, short ID and port);
- validates the config and the service before touching anything;
- registers the direct share link back to Volok over HTTPS;
- never stores the admin token on the VPS and never sends server private keys.

## Subscriptions for readers

```sh
volok --file /etc/volok/volok.json user add
# prints: user token: <TOKEN>
#         subscription: https://vpn.example.com/sub?token=<TOKEN>

volok --file /etc/volok/volok.json user list
volok --file /etc/volok/volok.json user remove <TOKEN> --yes
```

Every user token reads the same subscription of enabled nodes. User tokens
cannot install nodes or change the library.

## Access model

| Operation | Admin token | User token |
|---|---|---|
| `GET /sub` | no | yes |
| `GET /register` (installer) | yes | no |
| `PUT /nodes/{id}` (registration) | yes | no |
| Library/config changes | local CLI only | no |

## HTTPS

Volok listens on `127.0.0.1:41220` by default. Put an HTTPS reverse proxy
(domain + TLS) in front of it, point `public_url` at that origin, and make
sure the proxy does not cache `/sub`, `/register` or `/nodes/*` and does not
log token query strings.

## Important limitations

- **No relay:** a VPS that is unreachable directly stays unreachable; Volok
  only distributes links.
- **No revocation of issued links:** disabling/removing a node or deleting a
  user token stops future subscription updates, but already distributed
  VLESS links keep working (Xray on the VPS must be changed separately).
- **One shared subscription:** all user tokens see the same enabled nodes.
- Volok itself does not make network probes to your servers.

## OpenWrt

Build the `.ipk` (see `deploy/openwrt/`) or copy the binary to
`/usr/bin/volok`, place the library at `/etc/volok/volok.json`, then use
`volok service install`.

## Development

```sh
go test ./...
go vet ./...
golangci-lint run ./...
bash -n internal/installer/node.sh
```
