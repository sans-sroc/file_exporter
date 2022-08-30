package monitor

import (
	"context"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

	filePermissions = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "file_permissions",
		Help: "The permissions of a file",
	}, []string{"path"})

	fileContentHashCRC32 = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "file_content_hash_crc32",
		Help: "The CRC32 Hash of the file's content",
	}, []string{"path"})

	fileEvent = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "file_event",
		Help: "Events that occur against a file",
	}, []string{"path", "op"})

	filePendingPaths = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "file_pending_paths",
		Help: "Paths that are pending monitoring, usually because they were initially not found",
	})

	filePendingRecursivePaths = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "file_pending_recursive_paths",
		Help: "Paths that are pending monitoring, usually because they were initially not found",
	})

	pendingPaths          = []string{}
	pendingRecursivePaths = []string{}
)

func New(ctx context.Context, c *cli.Context, log *logrus.Logger) error {
	logentry := log.WithField("component", "monitor")

	w := watcher.New()

	// Only notify rename and move events.
	// w.FilterOps(watcher.Rename, watcher.Move)

	// Only files that match the regular expression during file listings
	// will be watched.
	// r := regexp.MustCompile("^abc$")
	// w.AddFilterHook(watcher.RegexFilterHook(r, false))

	if c.String("regex") != "" {
		r, err := regexp.Compile(c.String("regex"))
		if err != nil {
			return err
		}

		w.AddFilterHook(watcher.RegexFilterHook(r, c.Bool("regex-fullpath")))
	}

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

				metricPath = filepath.Clean(metricPath)
				metricPath = filepath.ToSlash(metricPath)

				logentry.WithField("path", metricPath).WithField("op", event.Op).Debug("event received")

				if event.Op == watcher.Remove {
					fileEvent.WithLabelValues(metricPath, event.Op.String()).Inc()

					fileContentHashCRC32.DeleteLabelValues(metricPath)
					fileStatModified.DeleteLabelValues(metricPath)
					filePermissions.DeleteLabelValues(metricPath)
				} else if event.Op == watcher.Rename {
					oldMetricPath := event.OldPath
					if c.String("rootfs") != "" {
						oldMetricPath = strings.ReplaceAll(oldMetricPath, c.String("rootfs"), "")
					}
					oldMetricPath = filepath.Clean(oldMetricPath)
					oldMetricPath = filepath.ToSlash(oldMetricPath)

					fileEvent.WithLabelValues(oldMetricPath, event.Op.String()).Inc()

					fileContentHashCRC32.DeleteLabelValues(oldMetricPath)
					fileStatModified.DeleteLabelValues(oldMetricPath)
					filePermissions.DeleteLabelValues(oldMetricPath)

					generateMetrics(metricPath, c.String("rootfs"))
				} else {
					fileEvent.WithLabelValues(metricPath, event.Op.String(), "").Inc()
					generateMetrics(metricPath, c.String("rootfs"))
				}
			case err := <-w.Error:
				if err == watcher.ErrWatchedFileDeleted {
					if len(c.String("paths")) > 0 {
						addWatcherPaths(w, logentry, c.String("rootfs"), strings.Split(c.String("paths"), ","))
					}

					if len(c.StringSlice("path")) > 0 {
						addWatcherPaths(w, logentry, c.String("rootfs"), c.StringSlice("path"))
					}
				}
				logentry.WithError(err).Error("watch error")
			case <-w.Closed:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		filePendingPaths.Set(float64(len(pendingPaths)))
		filePendingRecursivePaths.Set(float64(len(pendingRecursivePaths)))

		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
				logentry.Debug("processing pending paths")
				for i, path := range pendingPaths {
					if err := w.Add(path); err != nil {
						logentry.WithField("path", path).WithError(err).Error("unable to add path for watching (pending-path, retry)")
						continue
					}
					logentry.WithField("path", path).Debug("successfully adding pending path")
					pendingPaths = append(pendingPaths[:i], pendingPaths[i+1:]...)
				}

				for i, path := range pendingRecursivePaths {
					if err := w.AddRecursive(path); err != nil {
						logentry.WithField("path", path).WithError(err).Error("unable to add file for watching (pending-recursive-path, retry)")
						continue
					}
					logentry.WithField("path", path).Info("successfully adding pending path")
					pendingRecursivePaths = append(pendingRecursivePaths[:i], pendingRecursivePaths[i+1:]...)
				}

				rootfs := c.String("rootfs")
				runWatchedFiles(w, logentry, rootfs)

				filePendingPaths.Set(float64(len(pendingPaths)))
				filePendingRecursivePaths.Set(float64(len(pendingRecursivePaths)))
			}
		}
	}()

	if len(c.String("paths")) > 0 {
		addWatcherPaths(w, logentry, c.String("rootfs"), strings.Split(c.String("paths"), ","))
	}

	addWatcherPaths(w, logentry, c.String("rootfs"), c.StringSlice("path"))

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
				pendingRecursivePaths = append(pendingRecursivePaths, path)
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
			pendingRecursivePaths = append(pendingRecursivePaths, path)
			logentry.WithError(err).WithField("path", path).Error("unable to add directory for recursive watch")
		}
	}

	rootfs := c.String("rootfs")
	runWatchedFiles(w, logentry, rootfs)

	// Start the watching process - it'll check for changes every 5 seconds.
	if err := w.Start(time.Second * 5); err != nil {
		logentry.Error(err)
	}

	return nil
}

func addWatcherPaths(w *watcher.Watcher, logentry *logrus.Entry, rootfs string, paths []string) {
	for _, f := range paths {
		path := filepath.Join(rootfs, f)
		abs, err := filepath.Abs(path)
		if err != nil {
			logentry.WithError(err).Error("unable to get abs path")
		} else {
			path = abs
		}

		logentry.WithField("path", path).Debug("monitored path from paths")
		if err := w.Add(f); err != nil {
			pendingPaths = append(pendingPaths, path)
			logentry.WithField("path", path).WithError(err).Error("unable to add file for watching")
		}
	}
}

func runWatchedFiles(w *watcher.Watcher, logentry *logrus.Entry, rootfs string) {
	logrus.Debug("processing all watched files")
	for path, f := range w.WatchedFiles() {
		path = filepath.Clean(path)
		path = filepath.ToSlash(path)

		if f.IsDir() {
			continue
		}

		logentry.WithField("path", path).Debug("watched file")

		generateMetrics(path, rootfs)
	}
}

func generateMetrics(path string, rootfs string) {
	metricPath := path
	if rootfs != "" {
		metricPath = strings.ReplaceAll(metricPath, rootfs, "")
	}

	metricPath = filepath.Clean(metricPath)
	metricPath = filepath.ToSlash(metricPath)

	fileStatModified.WithLabelValues(metricPath).SetToCurrentTime()

	crc32, err := generateCRC32(path)
	if err != nil {
		logrus.WithError(err).Error("unable to generate crc32")
		return
	}

	fileContentHashCRC32.WithLabelValues(metricPath).Set(float64(*crc32))

	stats, err := os.Stat(path)
	if err != nil {
		logrus.WithError(err).Error("unable to get file stats")
		return
	}

	perms := fmt.Sprintf("%#o", stats.Mode().Perm())
	i, err := strconv.Atoi(perms)
	if err != nil {
		logrus.WithError(err).Error("unable to convert string to int")
		return
	}

	filePermissions.WithLabelValues(metricPath).Set(float64(i))
}

func generateCRC32(path string) (*uint32, error) {
	hash := crc32.NewIEEE()

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

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
