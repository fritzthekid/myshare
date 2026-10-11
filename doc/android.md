# webshare on Android (Termux)

`webshare` doesn't run on a server, but **directly on the phone itself**,
in [Termux](https://termux.dev/) (a terminal emulator with a real Linux
environment for Android - install from **F-Droid**, not the outdated
Play Store version). Since the Nextcloud access then originates from a
real server process (not from the browser), neither CORS nor reachability
on the LAN matter - it's enough for the phone to be able to reach
Nextcloud (WiFi, mobile network, anywhere).

Why not `web/share.html` (purely client-side, no Termux needed)? Because
that requires the Nextcloud instance to allow CORS for browser access -
default setups usually don't, and without admin access to the
Nextcloud/reverse proxy server, that can't be retrofitted (see comments in
`web/share.html`). Why not `cmd/webshare` on a server? Because that
requires a permanently reachable server on the same network/VPN.

## Important pitfall: DNS

A Go binary cross-compiled from a Linux machine (regardless of whether
`GOOS=linux` or `GOOS=android`, each with `CGO_ENABLED=0`) **cannot**
resolve DNS on Android - Android has no normal `/etc/resolv.conf`-based
DNS, Go's pure-Go resolver then tries a request to a (non-existent) local
resolver and fails with `read: connection refused`.

An `/etc/hosts` entry with a fixed IP is **not a real solution** (DynDNS,
and/or the same IP serving multiple targets via SNI/virtual hosting - both
common with self-hosted instances).

The only robust way: **compile `webshare` directly in Termux itself** -
a C compiler is available there, Go then automatically enables cgo, which
correctly uses the real Android DNS (Bionic).

## Setup

**1) Get the source code onto the phone.** The repo is private - `git
clone` in Termux would first need GitHub credentials there. Simpler: just
package the needed source code and transfer it via your own `sendfile`
(dogfooding):

```bash
# on the Linux machine:
tar czf webshare-src.tar.gz go.mod go.sum cmd/webshare internal
sudo bash -c 'set -a; . /etc/myshare/backend.env; set +a; ./sendfile webshare-src.tar.gz webshare-src.tar.gz'
# -> open the link/QR code on the phone, the file lands in ~/storage/downloads/
```

**2) Extract and build in Termux:**

```bash
termux-setup-storage                       # once, allow access to Downloads
cp ~/storage/downloads/webshare-src.tar.gz ~/
tar xzf ~/webshare-src.tar.gz -C ~         # extracts go.mod, go.sum, cmd/, internal/ into ~

pkg install golang clang -y                # once, takes a while
go build -o ~/webshare-native ./cmd/webshare
file ~/webshare-native                     # should show "dynamically linked" (cgo active)
```

**3) Credentials + token** (a file of Termux's own, no root/sudo needed -
Termux's own app storage is already isolated from other apps by Android):

```bash
cat > ~/webshare.env <<'ENV'
export NC_BASE_URL=https://cloud.example.com
export NC_USERNAME=change-cloud-user
export NC_PASSWORD=<app-password>
export WEBSHARE_TOKEN=<generate with "openssl rand -hex 24">
export WEBSHARE_LISTEN=127.0.0.1:8642
ENV
chmod 600 ~/webshare.env
```

**4) Start** (in the foreground, for testing):

```bash
source ~/webshare.env
~/webshare-native
# -> in the phone browser: http://127.0.0.1:8642/s/<token>/
```

Tested live against a real Nextcloud instance (upload + share link + QR
code display in the phone browser) - works.

## Running in the background

**Simple** (survives app switching, but not closing Termux/a reboot):

```bash
termux-wake-lock
source ~/webshare.env
nohup ~/webshare-native > ~/webshare.log 2>&1 &
disown
```

Check/stop: `pgrep webshare-native`, `tail -f ~/webshare.log`,
`pkill webshare-native`, `termux-wake-unlock`.

**With termux-services (restart on crash + auto-start the next time
Termux is opened):**

```bash
pkg install termux-services -y
# Restart Termux completely (close the app + reopen it), so the service
# supervisor (runsvdir) becomes active

mkdir -p $PREFIX/var/service/webshare
cat > $PREFIX/var/service/webshare/run <<'RUN'
#!/data/data/com.termux/files/usr/bin/sh
set -a
. /data/data/com.termux/files/home/webshare.env
set +a
exec /data/data/com.termux/files/home/webshare-native
RUN
chmod +x $PREFIX/var/service/webshare/run

sv-enable webshare
sv status webshare   # should show "run: webshare: (pid ...) ...s"
```

This makes `runsvdir` automatically start/supervise the service on every
new Termux session and restart it on a crash.

**No real reboot automation possible:** The obvious way to do this would
be [Termux:Boot](https://f-droid.org/packages/com.termux.boot/) (runs
scripts on phone boot) - but this app doesn't exist for every Android
version (not available on this device, for example). Without Termux:Boot,
**nothing starts on its own** after a phone restart - the service only
runs again once Termux is opened the next time (then `runsvdir`/
`sv-enable` takes over automatically, no manual start command needed).
For occasional use, that's usually enough; for "runs permanently in the
background, even without ever opening Termux" you'd need a different
automation app (e.g. Tasker/MacroDroid with the Termux:Task plugin) - not
pursued further here.

### Implemented

Open Termux
~~~
ls ~
webshare-native
webshare.env
$ 
$ mkdir -p ~/.termux/service/webshare/
$ cat > ~/.termux/service/webshare/run << 'EOF'
#!/data/data/com.termux/files/usr/bin/sh
. ~/webshare.env
exec ~/webshare-native
EOF
$ 
$ cat > ~/.termux/service/webshare/log/run << 'EOF'
#!/data/data/com.termux/files/usr/bin/sh
exec svlogd -tt ~/.termux/var/log/sv/webshare
EOF
$ 
$ chmod +x ~/.termux/service/webshare/log/run
$ mkdir -p ~/.termux/var/log/sv/webshare
$ 
$ cat >> .bashrc << 'EOF'
if ! pgrep -f "runsvdir /data/data/com.termux/files/home/.termux/service" > /dev/null; then
    runsvdir ~/.termux/service > ~/.termux/var/log/runsvdir.log 2>&1 &
fi
EOF
$
~~~

<!-- Add your own notes/additions from practice here, e.g. whether the
     reboot test actually worked, battery optimization exceptions that
     were needed, etc. -->
