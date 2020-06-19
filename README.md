# rssaws
Create a list for scraping AWS Service Health Dashboard and registering RSS Feed to Slack.

## Description
- Output the file to register AWS RSS Feed in Slack.
- Scraping the following web pages to get RSS Feed URL.
- The acquired URL list is output to a file by organizing it by the region and service.

***see also:***

- AWS Service Health Dashboard 
	- https://status.aws.amazon.com

## Features
- It is made by golang so it supports multi form.
- Get output organized by region and service.
- You can control the operation in the setting file.
	- You can specify the URL of the scraping destination.

## Requirement
- Go 1.14+
- Packages in use
	- PuerkitoBio/goquery: A little like that j-thing, only in Go.
		- https://github.com/PuerkitoBio/goquery
	- tidwall/gjson: Get JSON values quickly - JSON parser for Go
		- https://github.com/tidwall/gjson

## Usage
Just run the only one command.

```	sh
$ ./rssaws
```

However, setting is necessary to execute.

### Setting Example

1. In the same place as the binary file create execution settings file.

1. Execution settings are done with `config.json` file.

```sh
{
	"RssURL": "https://status.aws.amazon.com/",
	"RegionsJSON": "https://ip-ranges.amazonaws.com/ip-ranges.json"
}
```

- About setting items
	- `RssURL `: String
		- Specify the URL that can be acquired by RSS Feed of AWS service status.
	- `RegionsJSON `: String
		- Specify the URL of the AWS IP list to get the region name.
			- AWS IP address ranges
				- https://docs.aws.amazon.com/general/latest/gr/aws-ip-ranges.html

**-> Please edit the output result file directly if you want to do fine editing.**

## Installation

If you build from source yourself.

```	console
$ go get -u github.com/uchimanajet7/rssaws
$ go build
```

### When you want to check the output result
Download the sample output results files `sample_region_feed.txt` or `sample_service_feed.txt` from the repository.

- rssaws/sample_region_feed.txt at master · uchimanajet7/rssaws 
	- https://github.com/uchimanajet7/rssaws/blob/master/sample_region_feed.txt
- rssaws/sample_service_feed.txt at master · uchimanajet7/rssaws 
	- https://github.com/uchimanajet7/rssaws/blob/master/sample_service_feed.txt

**-> Please note that it may not be the latest information.**

## Author
[uchimanajet7](https://github.com/uchimanajet7)

## Licence
[Apache License 2.0](https://github.com/uchimanajet7/rssaws/blob/master/LICENSE)

## As reference information
- AWS Service Health Dashboard のRSS Feed をSlack に自動登録する #aws #slack - uchimanajet7のメモ
	- Under construction.
