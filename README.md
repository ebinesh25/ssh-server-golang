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
cat about.json
cat contact.json
cd blogs
ls
cat from-training-models-to-editing-their-weights.md
```

Available commands are `ls [path]`, `cd [path]`, `pwd`, `cat <file>`, `help`, `clear`, and `exit`. Blog articles show their title, summary, reading time, and a full URL that terminal clients can open as a link.

The portfolio JSON is fetched once when the server starts. If that request fails, the server stays available with basic identity and contact files. Restart the server to pick up changes from the website.

The server creates `ssh_host_ed25519` on first launch and reuses it on later launches. Set `SSH_HOST_KEY` to store it elsewhere. Keep that file between restarts so SSH clients continue to recognize the server.

## Build

```sh
go build -o minimal-ssh-server .
./minimal-ssh-server
```

This experiment does not configure user authentication. Keep it on a trusted network and behind the appropriate access controls before exposing it publicly.
