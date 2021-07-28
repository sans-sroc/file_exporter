package commands

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/sans-sroc/file_exporter/pkg/common"
	"github.com/sans-sroc/file_exporter/pkg/monitor"
)

const serviceName = "file_exporter"

type apiServerCommand struct{}

func (s *apiServerCommand) Execute(c *cli.Context) error {
	ctx := signals.SetupSignalHandler(context.Background())

	p1 := c.StringSlice("path")
	p2 := c.StringSlice("recursive-path")
	p3 := strings.Split(c.String("paths"), ",")
	p4 := strings.Split(c.String("recursive-paths"), ",")

	if len(p1) == 0 && len(p2) == 0 && len(p3) == 0 && len(p4) == 0 {
		return errors.New("You must pass a path or path-recursive to the tool for monitoring")
	}

	globalLevel := logrus.GetLevel()

	log := logrus.New()
	log.SetLevel(globalLevel)

	serviceCtx, err := runService(ctx, log)
	if err != nil {
		return err
	}

	go monitor.New(serviceCtx, c, log)

	listen := c.String("telemetry.addr")
	entry := log.WithField("component", "metrics").WithField("telemetry.addr", listen)

	router := mux.NewRouter().StrictSlash(true)
	router.Path("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html>
<head><title>file_exporter</title></head>
<body>
<h1>file_exporter</h1>
<p><a href="` + c.String("telemetry.path") + `">Metrics</a></p>
<p><i>` + common.AppVersion.Summary + `</i></p>
</body>
</html>`))
	})

	router.Path("/metrics").Handler(promhttp.Handler())

	srv := &http.Server{
		Addr:    listen,
		Handler: router,
	}

	go func() {
		entry.Info("Starting Metrics Server")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			entry.WithError(err).Error("an error occurred with metrics server")
		}
	}()

	<-serviceCtx.Done()

	entry.Info("Shutting down metrics server")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	srv.SetKeepAlivesEnabled(false)
	if err := srv.Shutdown(ctx); err != nil {
		entry.Fatalf("Could not gracefully shutdown the metrics server: %v\n", err)
	}

	return nil
}

func init() {
	cmd := apiServerCommand{}

	flags := []cli.Flag{
		&cli.StringFlag{
			Name:    "telemetry.addr",
			Usage:   "Host and port to listen on",
			EnvVars: []string{"TELEMTRY_ADDR"},
			Value:   "0.0.0.0:9183",
		},
		&cli.StringFlag{
			Name:    "telemetry.path",
			Usage:   "Path to listen for telemetry on",
			EnvVars: []string{"TELEMETRY_PATH"},
			Value:   "/metrics",
		},
		&cli.StringSliceFlag{
			Name:    "path",
			Usage:   "Path to monitor, will not be recursed",
			Aliases: []string{"p"},
			EnvVars: []string{"SINGLE_PATH"},
		},
		&cli.StringSliceFlag{
			Name:    "recursive-path",
			Usage:   "Path to monitor with recursion",
			Aliases: []string{"rp"},
			EnvVars: []string{"RECURSIVE_PATH"},
		},
		&cli.StringFlag{
			Name:    "paths",
			Usage:   "Paths to monitor, comma separated (will not be recursed)",
			EnvVars: []string{"PATHS"},
			Hidden:  true,
		},
		&cli.StringFlag{
			Name:    "recursive-paths",
			Usage:   "Paths to monitor recursively, comma separated (will not be recursed)",
			EnvVars: []string{"RECURSIVE_PATHS"},
			Hidden:  true,
		},
		&cli.StringFlag{
			Name:    "rootfs",
			Usage:   "Location of the root fs",
			EnvVars: []string{"ROOTFS"},
		},
	}

	cliCmd := &cli.Command{
		Name:   "server",
		Usage:  "server",
		Action: cmd.Execute,
		Flags:  append(flags, globalFlags()...),
		Before: globalBefore,
	}

	common.RegisterCommand(cliCmd)
}
