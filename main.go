package main

import (
	"bufio"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/tidwall/gjson"
)

// refer to:
// https://golang.org/src/net/mail/message.go?h=debugT#L36
var debug = debugT(false)

type debugT bool

func (d debugT) Printf(format string, args ...interface{}) {
	if d {
		log.Printf(format, args...)
	}
}

type userConfig struct {
	RssURL      string
	RegionsJSON string
}

func (c *userConfig) loadConfig() error {
	dir, err := getExecDir()
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "config.json")

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(c)

	return err
}

func (c *userConfig) getRssURLs() (map[string]string, error) {
	urlMap := make(map[string]string)

	// Parse URL
	parsedURL, err := url.Parse(c.RssURL)
	if err != nil {
		return urlMap, err
	}
	debug.Printf("parsedURL: %+v", parsedURL)

	// Load target RSS
	doc, err := goquery.NewDocument(parsedURL.String())
	if err != nil {
		return urlMap, err
	}
	debug.Printf("doc: %+v", doc)

	// Get service name and RSS URL
	var nameStr string
	doc.Find("tr > td").Each(func(_ int, s *goquery.Selection) {
		// service name
		if s.HasClass("bb top pad8") || s.HasClass("bb pad8 top") {
			nameStr = s.Text()
			debug.Printf("name: %+v", nameStr)
			return
		}

		// RSS URL
		val, exists := s.Find("a").Attr("href")

		debug.Printf("href: %+v", exists)
		debug.Printf("val: %+v", val)
		debug.Printf("rss: %+v", strings.HasSuffix(val, ".rss"))

		if !exists {
			debug.Printf("####### %+v", "none href")
			return
		}
		if !strings.HasSuffix(val, ".rss") {
			debug.Printf("####### %+v", "none .rss")
			return
		}

		rssURL := *parsedURL
		rssURL.Path = path.Join(rssURL.Path, strings.TrimSpace(val))
		urlMap[rssURL.String()] = nameStr
		debug.Printf("rssURL: %+v", rssURL.String())
	})
	debug.Printf("urls: %+v", urlMap)

	return urlMap, err
}

func (c *userConfig) getAwsRegions() (map[string]struct{}, error) {
	regionSet := make(map[string]struct{})

	// Parse URL
	parsedURL, err := url.Parse(c.RegionsJSON)
	if err != nil {
		return regionSet, err
	}
	debug.Printf("parsedURL: %+v", parsedURL)

	// Get JSON Data
	resp, err := http.Get(parsedURL.String())
	if err != nil {
		return regionSet, err
	}
	defer resp.Body.Close()

	// Ger JSON String
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return regionSet, err
	}
	debug.Printf("json: %+v", string(b))

	// Parse JSON
	result := gjson.GetBytes(b, "prefixes.#.region")
	for _, name := range result.Array() {
		regionSet[name.String()] = struct{}{}
	}
	// Add S3 US Standard
	// https://dev.classmethod.jp/articles/s3-consistency-update/
	regionSet["us-standard"] = struct{}{}

	debug.Printf("regionSet: %+v", regionSet)

	return regionSet, err
}

func main() {
	log.Println("------- start rssaws -------")
	startTime := time.Now()

	// Set default value
	userConf := &userConfig{
		RssURL:      "https://status.aws.amazon.com/",
		RegionsJSON: "https://ip-ranges.amazonaws.com/ip-ranges.json",
	}

	funcTime := time.Now()
	// Load user config
	err := userConf.loadConfig()
	debug.Printf("userConf: %+v", userConf)
	log.Printf("-> loadConfig(): %+v sec\n", (time.Now().Sub(funcTime)).Seconds())

	funcTime = time.Now()
	// Get RSS URL list
	urlMap, err := userConf.getRssURLs()
	if err != nil {
		log.Fatalf("%+v", err)
	}
	log.Printf("-> urlCount: %+v", len(urlMap))
	log.Printf("-> getRssURLs(): %+v sec\n", (time.Now().Sub(funcTime)).Seconds())

	funcTime = time.Now()
	// Get AWS regions
	regionSet, err := userConf.getAwsRegions()
	if err != nil {
		log.Fatalf("%+v", err)
	}
	log.Printf("-> regionCount: %+v", len(regionSet))
	log.Printf("-> getAwsRegions(): %+v sec\n", (time.Now().Sub(funcTime)).Seconds())

	funcTime = time.Now()
	// Group by region
	regionOutput := collectByRegion(urlMap, regionSet)
	log.Printf("-> collectByRegion(): %+v sec\n", (time.Now().Sub(funcTime)).Seconds())

	funcTime = time.Now()
	// Group by Service
	serviceOutput, serviceMap := collectByService(urlMap, regionSet)
	log.Printf("-> collectByService(): %+v sec\n", (time.Now().Sub(funcTime)).Seconds())

	funcTime = time.Now()
	// Write by region
	err = writeSlackFeed("_region_feed.txt", regionOutput, nil)
	if err != nil {
		log.Fatalf("%+v", err)
	}
	log.Printf("-> writeRegionFeed: %+v sec\n", (time.Now().Sub(funcTime)).Seconds())

	funcTime = time.Now()
	// Write by service
	err = writeSlackFeed("_service_feed.txt", serviceOutput, serviceMap)
	if err != nil {
		log.Fatalf("%+v", err)
	}
	log.Printf("-> writeServiceFeed(): %+v sec\n", (time.Now().Sub(funcTime)).Seconds())

	log.Println("==========================")
	log.Printf("-> total: %+v sec\n", (time.Now().Sub(startTime)).Seconds())
	log.Println("------- end rssaws -------")
}

func collectByRegion(urlMap map[string]string, regionSet map[string]struct{}) map[string][]string {
	const GLOBAL = "GLOBAL"
	regionResults := make(map[string][]string, len(regionSet))
	usedKeys := make(map[string]struct{}, len(urlMap))

	// Group by region
	for regionKey := range regionSet {
		debug.Printf("regionKey: %+v", regionKey)

		if GLOBAL == regionKey {
			continue
		}

		for urlKey := range urlMap {
			debug.Printf("urlKey: %+v", urlKey)

			if strings.LastIndex(urlKey, regionKey) < 0 {
				continue
			}
			regionResults[regionKey] = append(regionResults[regionKey], urlKey)
			// Mark used items
			usedKeys[urlKey] = struct{}{}
		}
	}

	debug.Printf("usedKeys: %+v", usedKeys)

	// Collect regionless global services
	// refer to:
	// https://docs.aws.amazon.com/ja_jp/general/latest/gr/aws-ip-ranges.html#aws-ip-download
	for urlKey := range urlMap {
		_, ok := usedKeys[urlKey]
		if ok {
			continue
		}
		regionResults[GLOBAL] = append(regionResults[GLOBAL], urlKey)
	}
	debug.Printf("regionResults: %+v", regionResults)

	return regionResults
}

func collectByService(urlMap map[string]string, regionSet map[string]struct{}) (map[string][]string, map[string]string) {
	const GLOBAL = "GLOBAL"
	serviceMap := make(map[string]string, len(urlMap))
	usedKeys := make(map[string]struct{}, len(urlMap))
	serviceResults := make(map[string][]string, len(urlMap))

	// Group by service
	for urlKey, urlVal := range urlMap {
		debug.Printf("urlKey: %+v", urlKey)

		base := path.Base(urlKey)
		ext := path.Ext(urlKey)

		debug.Printf("base: %+v", base)
		debug.Printf("ext: %+v", ext)

		urlItem := strings.Replace(base, ext, "", -1)

		if urlItem == "" {
			continue
		}
		debug.Printf("urlItem: %+v", urlItem)

		for regionKey := range regionSet {
			if regionKey == GLOBAL {
				continue
			}
			regionItem := strings.Replace(urlItem, regionKey, "", -1)

			if regionItem == urlItem {
				continue
			}
			serviceMap[regionItem] = urlVal
			// Mark used items
			usedKeys[urlKey] = struct{}{}
			serviceResults[regionItem] = append(serviceResults[regionItem], urlKey)
		}
	}

	debug.Printf("usedKeys: %+v", usedKeys)

	for urlKey, urlVal := range urlMap {
		_, ok := usedKeys[urlKey]
		if ok {
			continue
		}
		base := path.Base(urlKey)
		ext := path.Ext(urlKey)

		urlItem := strings.Replace(base, ext, "", -1)
		if urlItem == "" {
			continue
		}
		debug.Printf("urlItem: %+v", urlItem)
		debug.Printf("urlVal: %+v", urlVal)

		serviceMap[urlItem] = urlVal
		serviceResults[urlItem] = append(serviceResults[urlItem], urlKey)
	}
	debug.Printf("serviceMap: %+v", serviceMap)
	debug.Printf("serviceResults: %+v", serviceResults)

	return serviceResults, serviceMap
}

func getExecDir() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}

	return filepath.Dir(execPath), nil
}

func getConfigPath() (string, error) {
	dir, err := getExecDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "config.json"), err
}

func writeSlackFeed(filename string, output map[string][]string, names map[string]string) error {
	dir, err := getExecDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, filename)

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Sort by map key
	sortedKeys := make([]string, len(output))
	for key := range output {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)

	// Save sorted data
	w := bufio.NewWriter(f)
	for _, val := range sortedKeys {
		urls, ok := output[val]
		if !ok {
			continue
		}
		text := strings.TrimSpace(val)

		// Edit the name output to the header
		if names != nil && len(names) > 0 {
			name, ok := names[val]
			if ok {
				text = name
				// Remove parenthesized strings
				// ex.) AWS VPCE PrivateLink (Singapore)
				index := strings.Index(name, "(")
				if index > -1 {
					text = name[:strings.Index(name, "(")]
				}
				text = strings.TrimSpace(text)
			}
		}

		// Write comment header
		text = "# " + text + "\n"
		_, err = w.WriteString(text)
		if err != nil {
			return err
		}

		// Sort by url
		sort.Strings(urls)
		for _, item := range urls {
			// Write in slack feed app format
			// https://slack.com/intl/ja-jp/help/articles/218688467-Slack-%E3%81%AB-RSS-%E3%83%95%E3%82%A3%E3%83%BC%E3%83%89%E3%82%92%E8%BF%BD%E5%8A%A0%E3%81%99%E3%82%8B
			text = "/feed subscribe " + item + "\n"
			_, err = w.WriteString(text)
			if err != nil {
				return err
			}
		}

		// Write the end of this block
		text = "# \n"
		_, err = w.WriteString(text)
		if err != nil {
			return err
		}
	}
	w.Flush()

	return err
}
