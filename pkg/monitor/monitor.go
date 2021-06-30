package monitor

import (
	"context"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/radovskyb/watcher"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	fileStatModified = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "file_stat_modified_time_seconds",
		Help: "The unix time the file was last modified",
	}, []string{"path"})

	fileContentHashCRC32 = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "file_content_hash_crc32",
		Help: "The CRC32 Hash of the file's content",
	}, []string{"path"})

	fileEvent = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "file_event",
		Help: "Events that occur against a file",
	}, []string{"path", "op"})
)

func New(ctx context.Context, c *cli.Context, log *logrus.Logger) {
	logentry := log.WithField("component", "monitor")

	w := watcher.New()

	// Only notify rename and move events.
	// w.FilterOps(watcher.Rename, watcher.Move)

	// Only files that match the regular expression during file listings
	// will be watched.
	// r := regexp.MustCompile("^abc$")
	// w.AddFilterHook(watcher.RegexFilterHook(r, false))

	go func() {
		for {
			select {
			case event := <-w.Event:
				if event.IsDir() {
					continue
				}

				metricPath := event.Path
				if c.String("rootfs") != "" {
					metricPath = strings.ReplaceAll(metricPath, c.String("rootfs"), "")
				}

				fileEvent.WithLabelValues(metricPath, event.Op.String()).Inc()

				if event.Op == watcher.Remove {
					fileContentHashCRC32.DeleteLabelValues(metricPath)
					fileStatModified.DeleteLabelValues(metricPath)
				} else {
					fileEvent.WithLabelValues(metricPath, event.Op.String()).Inc()
					generateMetrics(event.Path, c.String("rootfs"))
				}

			case err := <-w.Error:
				logentry.Fatalln(err)
			case <-w.Closed:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	if len(c.String("paths")) > 0 {
		for _, f := range strings.Split(c.String("paths"), ",") {
			path := filepath.Join(c.String("rootfs"), f)
			abs, err := filepath.Abs(path)
			if err != nil {
				logentry.WithError(err).Error("unable to get abs path")
			} else {
				path = abs
			}

			logentry.WithField("path", path).Debug("monitored path from paths")
			if err := w.Add(f); err != nil {
				logentry.WithField("path", path).WithError(err).Error("unable to add file for watching")
			}
		}
	}

	for _, f := range c.StringSlice("path") {
		path := filepath.Join(c.String("rootfs"), f)
		abs, err := filepath.Abs(path)
		if err != nil {
			logentry.WithError(err).Error("unable to get abs path")
		} else {
			path = abs
		}

		logentry.WithField("path", path).Debug("monitored path flag")
		if err := w.Add(path); err != nil {
			logentry.WithField("path", path).WithError(err).Error("unable to add file for watching")
		}
	}

	if len(c.String("recursive-paths")) > 0 {
		for _, f := range strings.Split(c.String("recursive-paths"), ",") {
			path := filepath.Join(c.String("rootfs"), f)
			abs, err := filepath.Abs(path)
			if err != nil {
				logentry.WithError(err).Error("unable to get abs path")
			} else {
				path = abs
			}

			logentry.WithField("path", path).Debug("monitored path recursively")
			if err := w.AddRecursive(path); err != nil {
				logentry.WithField("path", path).WithError(err).Error("unable to add file for watching")
			}
		}
	}

	for _, d := range c.StringSlice("recursive-path") {
		path := filepath.Join(c.String("rootfs"), d)
		abs, err := filepath.Abs(path)
		if err != nil {
			logentry.WithError(err).Error("unable to get abs path")
		} else {
			path = abs
		}

		logentry.WithField("path", path).Debug("recursive path monitor")
		if err := w.AddRecursive(path); err != nil {
			logentry.WithError(err).WithField("path", path).Error("unable to add directory for recursive watch")
		}
	}

	rootfs := c.String("rootfs")

	for path, f := range w.WatchedFiles() {
		path = filepath.Clean(path)
		path = filepath.ToSlash(path)

		logentry.WithField("path", path).Debug("watched file")

		if f.IsDir() {
			continue
		}

		generateMetrics(path, rootfs)
	}

	// Start the watching process - it'll check for changes every 100ms.
	if err := w.Start(time.Second * 5); err != nil {
		logentry.Fatalln(err)
	}
}

func generateMetrics(path string, rootfs string) {
	metricPath := path
	if rootfs != "" {
		metricPath = strings.ReplaceAll(metricPath, rootfs, "")
	}

	fileStatModified.WithLabelValues(metricPath).SetToCurrentTime()

	crc32, err := generateCRC32(path)
	if err != nil {
		logrus.WithError(err).Error("unable to generate crc32")
		return
	}

	fileContentHashCRC32.WithLabelValues(metricPath).Set(float64(*crc32))
}

func generateCRC32(path string) (*uint32, error) {
	hash := crc32.NewIEEE()

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	// read chunks of 32k
	buf := make([]byte, 32*1024)

	for {
		c, err := file.Read(buf)
		slice := buf[:c]
		if _, errHash := hash.Write(slice); errHash != nil {
			return nil, err
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			logrus.Debugln("Error reading content of file", path, "-", err)
			return nil, err
		}
	}

	val := hash.Sum32()

	return &val, nil
}
