package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/digitalocean/godo"
	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	"golang.org/x/oauth2"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	doLabel          = model.MetaLabelPrefix + "do_"
	doLabelID        = doLabel + "id"
	doLabelName      = doLabel + "name"
	doLabelTags      = doLabel + "tags"
	doLabelStatus    = doLabel + "status"
	doLabelPrivateIP = doLabel + "private_ip"
	doLabelPublicIP  = doLabel + "public_ip"
	doLabelRegion    = doLabel + "region"
	doLabelSize      = doLabel + "size"
)

var (
	a           = kingpin.New("prometheus-digitalocean-sd", "Tool to generate file_sd target files from digitalocean.")
	outputFile  = a.Flag("output.file", "Output file for file_sd compatible file.").Default("do_sd.json").String()
	doToken     = a.Flag("token", "DigitalOcean API token").Envar("DO_TOKEN").Required().String()
	servicePort = a.Flag("service.port", "port to try to use on droplets found").Default("9100").String()
	sleep       = a.Flag("sleep", "time to sleep between each refresh").Default("1m").Duration()

	version = "master"
)

func main() {
	a.HelpFlag.Short('h')
	a.Version(version)
	a.VersionFlag.Short('v')
	kingpin.MustParse(a.Parse(os.Args[1:]))

	var ctx = context.Background()
	var client = godo.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(
		&oauth2.Token{
			AccessToken: *doToken,
		},
	)))

	for {
		if err := pullAndWrite(ctx, client); err != nil {
			log.Fatalln(err)
		}
		time.Sleep(*sleep)
	}
}

func pullAndWrite(ctx context.Context, client *godo.Client) error {
	log.Println("gathering droplets...")
	opt := &godo.ListOptions{
		Page: 1,
	}
	var nodes []godo.Droplet
	for {
		droplets, resp, err := client.Droplets.List(ctx, opt)
		if err != nil {
			return errors.Wrap(err, "could not  get the list of droplets")
		}
		nodes = append(nodes, droplets...)
		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}
		opt.Page = opt.Page + 1
	}
	targets, err := toTargetList(nodes)
	if err != nil {
		return err
	}
	return write(targets)
}

func toTargetList(nodes []godo.Droplet) ([]Target, error) {
	var targets []Target

	for _, node := range nodes {
		if len(node.Networks.V4) == 0 {
			continue
		}
		ipAddr, err := node.PublicIPv4()
		if err != nil {
			return targets, errors.Wrap(err, "could not  find a public ipv4 addr")
		}
		pvtIPAddr, err := node.PrivateIPv4()
		if err != nil {
			return targets, errors.Wrap(err, "could not  find a private ipv4 addr")
		}

		labels := model.LabelSet{
			doLabelID: model.LabelValue(fmt.Sprintf("%d", node.ID)),
		}
		if ipAddr != "" {
			labels[doLabelPublicIP] = model.LabelValue(ipAddr)
		}
		if pvtIPAddr != "" {
			labels[doLabelPrivateIP] = model.LabelValue(pvtIPAddr)
		}
		var addr = net.JoinHostPort(ipAddr, *servicePort)
		labels[doLabelStatus] = model.LabelValue(node.Status)
		labels[doLabelRegion] = model.LabelValue(node.Region.Slug)
		labels[doLabelSize] = model.LabelValue(node.SizeSlug)
		labels[doLabelName] = model.LabelValue(node.Name)
		labels[doLabelTags] = model.LabelValue(strings.Join(node.Tags, ","))
		targets = append(targets, Target{
			Targets: []string{addr},
			Labels:  labels,
		})
	}
	return targets, nil
}

func write(data []Target) error {
	b, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return errors.Wrap(err, "could not  marshal json")
	}

	dir, _ := filepath.Split(*outputFile)
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return errors.Wrap(err, "could not  create directory")
		}
	}
	tmpfile, err := ioutil.TempFile(dir, "sd")
	if err != nil {
		return errors.Wrap(err, "could not  create temp file")
	}
	defer tmpfile.Close() // nolint: errcheck

	_, err = tmpfile.Write(b)
	if err != nil {
		return errors.Wrap(err, "could not  write to temp file")
	}
	defer log.Println("written", *outputFile)
	return os.Rename(tmpfile.Name(), *outputFile)
}

// Target is a target that marshal into the file_sd prometheus json format
type Target struct {
	Targets []string       `json:"targets"`
	Labels  model.LabelSet `json:"labels"`
}
