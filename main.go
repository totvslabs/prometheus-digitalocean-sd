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
	"time"

	"github.com/digitalocean/godo"
	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	"golang.org/x/oauth2"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	doLabel                  = model.MetaLabelPrefix + "do_"
	doLabelInstanceID        = doLabel + "id"
	doLabelInstanceName      = doLabel + "name"
	doLabelInstanceStatus    = doLabel + "status"
	doLabelInstancePrivateIP = doLabel + "private_ip"
	doLabelInstancePublicIP  = doLabel + "public_ip"
	doLabelInstanceRegion    = doLabel + "az"
	doLabelInstanceSize      = doLabel + "size"
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
	a.Parse(os.Args[1:])
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
			return errors.Wrap(err, "couldn't get the list of droplets")
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
			return targets, errors.Wrap(err, "couldn't find a public ipv4 addr")
		}
		pvtIPAddr, err := node.PrivateIPv4()
		if err != nil {
			return targets, errors.Wrap(err, "couldn't find a private ipv4 addr")
		}

		labels := model.LabelSet{
			doLabelInstanceID: model.LabelValue(fmt.Sprintf("%d", node.ID)),
		}
		if ipAddr != "" {
			labels[doLabelInstancePublicIP] = model.LabelValue(ipAddr)
		}
		if pvtIPAddr != "" {
			labels[doLabelInstancePrivateIP] = model.LabelValue(pvtIPAddr)
		}
		var addr = net.JoinHostPort(ipAddr, fmt.Sprintf("%s", *servicePort))
		// labels[model.AddressLabel] = model.LabelValue(addr) do we need that?
		labels[doLabelInstanceStatus] = model.LabelValue(node.Status)
		labels[doLabelInstanceRegion] = model.LabelValue(node.Region.Slug)
		labels[doLabelInstanceSize] = model.LabelValue(node.SizeSlug)
		labels[doLabelInstanceName] = model.LabelValue(node.Name)
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
		return errors.Wrap(err, "couldn't marshal json")
	}

	dir, _ := filepath.Split(*outputFile)
	tmpfile, err := ioutil.TempFile(dir, "sd-adapter")
	if err != nil {
		return errors.Wrap(err, "couldn't create temp file")
	}
	defer tmpfile.Close()

	_, err = tmpfile.Write(b)
	if err != nil {
		return errors.Wrap(err, "couldn't write to temp file")
	}
	return os.Rename(tmpfile.Name(), *outputFile)
}

// Target is a target that marshal into the file_sd prometheus json format
type Target struct {
	Targets []string       `json:"targets"`
	Labels  model.LabelSet `json:"labels"`
}
