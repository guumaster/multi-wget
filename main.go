package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/urfave/cli/v2"
	"github.com/vbauerster/mpb/v5"
	"github.com/vbauerster/mpb/v5/decor"
)

/*
	Sample Usage:
	  multi-wget [URL1] [URL2]

*/

func main() {
	var urls []*url.URL

	_, err := exec.LookPath("wget")
	if err != nil {
		log.Fatal("can't find wget on your system")
	}

	outputPath, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	app := &cli.App{
		Name:  "multi-wget",
		Usage: "automate resumable downloads with wget",
		Before: func(c *cli.Context) error {
			list := c.Args().Slice()
			for _, u := range list {
				p, err := url.Parse(u)
				if err != nil {
					return fmt.Errorf("invalid url: %w", err)
				}
				urls = append(urls, p)
			}
			return nil
		},
		Action: func(c *cli.Context) error {
			var wg sync.WaitGroup

			pb := mpb.New(mpb.WithWaitGroup(&wg))
			wg.Add(len(urls))

			for _, downURL := range urls {
				go downloadMedia(&wg, pb, downURL, outputPath)
			}

			pb.Wait()
			return nil
		},
	}

	err = app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func downloadMedia(wg *sync.WaitGroup, pb *mpb.Progress, downloadUrl *url.URL, outputPath string) {
	defer wg.Done()
	wgetPath, _ := exec.LookPath("wget")

	hash, ok := downloadUrl.Query()["hash"]

	if !ok || len(hash) == 0 {
		hash = []string{path.Base(path.Clean(downloadUrl.Path))}
	}
	if hash[0] == "" {
		log.Fatal("can't calculate a file name")
	}

	outputFile := path.Join(outputPath, hash[0])
	args := []string{"-c", "-L", "--no-check-certificate", "-O", outputFile, downloadUrl.String()}

	cmd := exec.Command(wgetPath, args...)
	//	cmd.Stdout = os.Stdout
	//	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		log.Fatal(err)
	}

	go showProgressBar(downloadUrl.String(), outputFile, pb)

	err = cmd.Wait()

	if err != nil {
		log.Printf("finished with error: %v for URL: %s", err, downloadUrl)
	}
}

func showProgressBar(url, outputFile string, pb *mpb.Progress) {
	client := &http.Client{}

	res, err := client.Get(url)
	if err != nil {
		log.Fatal(err)
	}

	totalSize, _ := strconv.Atoi(res.Header["Content-Length"][0])
	bar := pb.AddBar(int64(totalSize),
		mpb.PrependDecorators(decor.Counters(decor.UnitKiB, "% .1f / % .1f")),
		mpb.AppendDecorators(decor.Percentage()),
	)

	last := 0
	for {
		fi, _ := os.Stat(outputFile)
		downloaded := int(fi.Size())
		bar.IncrBy(downloaded - last)
		last = downloaded
		if downloaded == totalSize {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
}
