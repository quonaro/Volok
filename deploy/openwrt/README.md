# Volok on OpenWrt

## Install

```sh
scp volok-linux-arm64 root@router:/usr/bin/volok
ssh root@router chmod +x /usr/bin/volok
scp deploy/openwrt/files/volok.init root@router:/etc/init.d/volok
ssh root@router chmod +x /etc/init.d/volok
```

## Configure

The init script does not create or rewrite the library. Initialize it once:

```sh
ssh root@router volok --file /etc/volok/volok.json init --public-url https://vpn.example.com
ssh root@router /etc/init.d/volok enable
ssh root@router /etc/init.d/volok start
```

`volok.json` (and its `.lock` file) live in `/etc/volok/`. Upgrades keep the
file; uninstalling the package does not delete it.

## HTTPS

Volok binds to `127.0.0.1:41230` by default. Terminate HTTPS on the same
router (or another trusted host) and proxy `/`, `/sub`, `/register`,
`/nodes/*` to it. Do not cache these paths and do not log token queries.

## Building the .ipk

The release workflow builds `volok-linux-*` binaries and the `.ipk`. The
`deploy/openwrt/Makefile` expects the binary named `volok` next to it and
pins the canonical init script from `internal/cli/assets/volok.init` (kept
identical to `deploy/openwrt/files/volok.init`).
