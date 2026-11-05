# Ebinesh Portfolio SSH

A small read-only SSH shell that presents the portfolio at `https://www.ebinesh.in/json/` as a virtual filesystem. It does not execute commands on the host.

## Run locally

```sh
go run .
```

The server listens on port `2222` by default. Connect from another terminal:

```sh
ssh -p 2222 localhost
```

To choose another port, set `PORT`:

```sh
PORT=2200 go run .
ssh -p 2200 localhost
```

## Explore

```text
ls /
tree /
cat about.json
cat contact.json
cd blogs
ls
cat from-training-models-to-editing-their-weights.md
cd ..
open blogs/from-training-models-to-editing-their-weights.md
```

The terminal opens with a colored welcome banner. Section names and directories are colored, and `tree` draws the full portfolio layout. `cd` supports absolute and relative paths, `..`, and `~` to return to the root. Use `sections` for a quick overview and `banner` to bring back the welcome card. Blog cards include clickable links in terminals that support OSC 8 hyperlinks, with the URL still visible for other clients.

Available commands are `sections`, `tree [path]`, `ls [path]`, `cd [path]`, `pwd`, `cat <file>`, `open <blog.md>`, `banner`, `help`, `clear`, and `exit`.

The portfolio JSON is fetched once when the server starts. If that request fails, the server stays available with basic identity and contact files. Restart the server to pick up changes from the website.

The server creates `ssh_host_ed25519` on first launch and reuses it on later launches. Set `SSH_HOST_KEY` to store it elsewhere. Keep that file between restarts so SSH clients continue to recognize the server.

## Build

```sh
go build -o minimal-ssh-server .
./minimal-ssh-server
```

This experiment does not configure user authentication. Keep it on a trusted network and behind the appropriate access controls before exposing it publicly.
