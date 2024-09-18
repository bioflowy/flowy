package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/bioflowy/flowy/golang/cmd/client/api"
	"github.com/urfave/cli/v2"
)

func showJobInfo(jobId string) error {
	cfg := api.NewConfiguration()
	cfg.Scheme = "http"
	cfg.Host = "127.0.0.1:5173"
	ctx := context.Background()
	c := api.NewAPIClient(cfg)
	res, _, err := c.DefaultAPI.ApiGetJobInfoPost(ctx).ApiGetJobInfoPostRequest(api.ApiGetJobInfoPostRequest{JobId: jobId}).Execute()
	if err != nil {
		return err
	}
	fmt.Printf("Status: %s\n", res.GetStatus())
	fmt.Printf("Results: %s\n", res.GetResult())
	return nil
}
func main() {
	app := &cli.App{
		Name:  "flowy-cmd",
		Usage: "Command line interface for flowy",
		Commands: []*cli.Command{
			{
				Name:  "job",
				Usage: "job",
				Subcommands: []*cli.Command{
					{
						Name:  "show",
						Usage: "show job infomation",
						Action: func(c *cli.Context) error {
							if c.NArg() > 0 {
								jobId := c.Args().Get(0)
								return showJobInfo(jobId)
							}
							return nil
						},
					},
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
