# LAN_DEPLOYMENT — KutubCore on a local network (Blueprint §2)

One host runs KutubCore; every phone/laptop on the same Wi-Fi/LAN opens
`http://<host>:8080` in the browser. The DB lives only on the host:
**never put the SQLite file on a network share** — clients must talk to the
HTTP API on the host.

## 1. Start the host (Linux)

```sh
git pull
go run ./cmd/kutubcore   # listens on :8080 (all interfaces by default)
```

For a specific port/host: `ADDR=0.0.0.0:8080 go run ./cmd/kutubcore`
(`ADDR` defaults to `:8080`, i.e. all interfaces).

The server logs a `lan_url` line with your LAN IP, e.g.
`http://192.168.1.20:8080`. Open that URL on any LAN device.

## 2. Discover the host by name (mDNS)

Most LANs already run mDNS via Avahi (Linux), Bonjour (macOS), or the OS.
If the host runs Avahi (`apt install avahi-daemon` on Debian/Ubuntu):

```sh
hostname        # e.g. library-pc
# browse: http://library-pc.local:8080
```

Find the IP without mDNS:

```sh
hostname -I    # LAN IPv4 of the host
```

## 3. Firewall

Allow the port only on the trusted LAN (not the WAN):

```sh
# ufw example (trusted LAN only)
sudo ufw allow from 192.168.1.0/24 to any port 8080 proto tcp
```

## 4. Auth over plain HTTP (LAN caveat)

LAN mode is trusted-network HTTP: `COOKIE_SECURE=false` (default). Session
cookies travel in cleartext on this network — only run LAN mode on a
network you trust. The server logs a warning when it binds 0.0.0.0 without
secure cookies on plain HTTP.

For HTTPS on the LAN, terminate TLS in a reverse proxy (below) and set
`COOKIE_SECURE=true` on the app.

## 5. Reverse proxy with HTTPS (recommended for permanent hosts)

```nginx
# nginx: proxy_pass http://127.0.0.1:8080; (with TLS certs via any ACME client)
# KutubCore then runs behind it with COOKIE_SECURE=true.
```

Caddy equivalent (2 lines, automatic HTTPS):

```
library.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

## 6. Data & backups on the host

- DB: `data/kutubcore.db` (SQLite), files: `data/files/`.
- Back up from the host: `./scripts/backup.sh` (see Blueprint §15).
- Keep one replica of `data/backups/` off-host (USB/cloud).
