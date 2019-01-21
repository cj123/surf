surf
====

a very simple application for automating snapshots of [DigitalOcean](https://digitalocean.com) droplets.

surf can be run multiple times per day and will back up only when the given interval has passed over its previous snapshot.

surf will **expire old snapshots** after a given duration, saving space in your account.

## install

`go get github.com/cj123/surf`

should work on go 1.7 and later.

## using it

copy surf.example.yml to surf.yml and edit it to your liking.

run surf as follows:

`SURF_CONFIG=/path/to/surf.yml surf`

## recommendations

1. use surf with cron
2. schedule surf's cronjob's with at least a 1 hour gap between.
3. ???
4. definitely not profit, but hopefully more useful backups than are currently afforded by DigitalOcean.