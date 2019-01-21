package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v2"
)

var surfConfigLocation = os.Getenv("SURF_CONFIG")

type SurfConfig struct {
	Droplets []*SurfDroplet `yaml:"droplets"`

	AccessToken string `yaml:"access_token"`
}

func (t *SurfConfig) Token() (*oauth2.Token, error) {
	token := &oauth2.Token{
		AccessToken: t.AccessToken,
	}
	return token, nil
}

type SurfDroplet struct {
	Name      string          `yaml:"name"`
	Snapshots []*SurfSnapshot `yaml:"snapshots"`

	droplet godo.Droplet
}

type SurfSnapshot struct {
	Interval time.Duration `yaml:"interval"`
	Keep     time.Duration `yaml:"keep"`
	Note     string        `yaml:"note"`
	PowerOff bool          `yaml:"poweroff"`
}

func checkError(what string, err error) {
	if err == nil {
		return
	}

	log.Fatalf("could not: %s, err: %s", what, err)
}

func main() {
	if surfConfigLocation == "" {
		surfConfigLocation = "surf.yml"
	}

	confFile, err := os.Open(surfConfigLocation)
	checkError("open config file", err)

	defer confFile.Close()

	var conf SurfConfig

	err = yaml.NewDecoder(confFile).Decode(&conf)
	checkError("parse config file", err)

	if len(conf.Droplets) == 0 {
		log.Println("no droplets configured!")
		return
	}

	oauthClient := oauth2.NewClient(context.Background(), &conf)
	digitalocean := godo.NewClient(oauthClient)

	ctx := context.Background()

	droplets, _, err := digitalocean.Droplets.List(ctx, nil)
	checkError("list droplets", err)

	for _, droplet := range conf.Droplets {
		for _, doDroplet := range droplets {
			if doDroplet.Name == droplet.Name {
				droplet.droplet = doDroplet
			}
		}
	}

	for _, droplet := range conf.Droplets {
		actions, _, err := digitalocean.Droplets.Actions(ctx, droplet.droplet.ID, nil)

		if err != nil {
			log.Printf("[%s] couldn't list actions for droplet, continuing (err: %s)", droplet.Name, err)
			continue
		}

		snapshotsInProgress := false

		for _, action := range actions {
			if action.Type == "snapshot" && action.Status == "in-progress" {
				snapshotsInProgress = true
				break
			}
		}

		if snapshotsInProgress {
			log.Printf("[%s] snapshot actions currently in progress for this droplet", droplet.Name)
			continue
		}

		// build a list of our last snapshots
		snapshots, _, err := digitalocean.Droplets.Snapshots(ctx, droplet.droplet.ID, nil)

		if err != nil {
			log.Printf("couldn't list snapshots for droplet: %s, continuing (err: %s)", droplet.Name, err)
			continue
		}

		for _, snapshotInfo := range droplet.Snapshots {
			var latestSnapshotTime time.Time

			for _, snapshot := range snapshots {
				if !strings.Contains(snapshot.Name, snapshotInfo.Note) {
					continue // this isn't the correct snapshot
				}

				created, err := time.Parse(time.RFC3339, snapshot.Created)

				if err != nil {
					log.Printf("[%s] cannot parse created date: %s", droplet.Name, err)
					continue
				}

				if created.After(latestSnapshotTime) {
					latestSnapshotTime = created
				}

				// check if it's out of date
				if time.Now().Sub(created) > snapshotInfo.Keep {
					log.Printf("[%s] found outdated snapshot: %s (%s), deleting...", droplet.Name, snapshot.Name, snapshot.Slug)

					// delete the snapshot
					_, err := digitalocean.Snapshots.Delete(ctx, fmt.Sprintf("%d", snapshot.ID))

					if err != nil {
						log.Printf("[%s] couldn't delete snapshot ID: %s, err: %s", droplet.Name, snapshot.Slug, err)
						continue
					}
				}
			}

			if time.Now().Sub(latestSnapshotTime) > snapshotInfo.Interval {
				if snapshotInfo.PowerOff {
					log.Printf("[%s] queuing power off before snapshot", droplet.Name)

					_, _, err := digitalocean.DropletActions.PowerOff(ctx, droplet.droplet.ID)

					if err != nil {
						log.Printf("[%s] could not power off before snapshot, err: %s", droplet.Name, err)
					}
				}

				log.Printf("[%s] queuing new snapshot for label: %s", droplet.Name, snapshotInfo.Note)

				_, _, err := digitalocean.DropletActions.Snapshot(ctx, droplet.droplet.ID, fmt.Sprintf("surf: %s at %s", snapshotInfo.Note, time.Now().Format(time.RFC3339)))

				if err != nil {
					log.Printf("[%s] couldn't create snapshot: %s", droplet.Name, err)
				}

				if snapshotInfo.PowerOff {
					log.Printf("[%s] queuing power on before snapshot", droplet.Name)

					_, _, err := digitalocean.DropletActions.PowerOn(ctx, droplet.droplet.ID)

					if err != nil {
						log.Printf("[%s] could not power off before snapshot, err: %s", droplet.Name, err)
					}
				}
			}
		}
	}
}
