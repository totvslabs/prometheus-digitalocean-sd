# prometheus-digitalocean-sd

Uses DigitalOcean's API to generate a `file_sd_config`'s compatible JSON
file, so you can service discovery DigitalOcean droplets on Prometheus.

## Usage

You can run it as docker container:

```sh
docker run \
  -v "$PWD":/tmp \
  -e DO_TOKEN="digitalocean-read-only-token" \
  --restart=always \
  totvslabs/prometheus-digitalocean-sd --output.file=/tmp/do_sd.json
```

And then change your `prometheus.yml` config file to use `file_sd_configs`
to read the `do_sd.json` file:

```yaml
scrape_configs:
  - job_name: 'node'
    file_sd_configs:
      - files: ['do_sd.json']
        refresh_interval: 30s
    relabel_configs:
      - source_labels: [__meta_do_status]
        regex: active
        action: keep
      - source_labels: [__meta_do_region]
        target_label: region
      - source_labels: [__meta_do_size]
        target_label: flavor
      - source_labels: [__meta_do_public_ip]
        target_label: ip
      - source_labels: [__meta_do_name]
        target_label: instance_name
```

## File format

The JSON file will have the following format and labels:

```json
[
	{
		"targets": [
			"1.2.3.4:9100"
		],
		"labels": {
			"__meta_do_az": "sfo1",
			"__meta_do_id": "1234",
			"__meta_do_name": "droplet-name",
			"__meta_do_public_ip": "1.2.3.4",
			"__meta_do_size": "4gb",
			"__meta_do_status": "active"
		}
	},
	// all other droplets
]
```
