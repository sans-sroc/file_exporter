package commands

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows/svc"

	el "github.com/sans-sroc/file_exporter/pkg/eventlog"
	"golang.org/x/sys/windows/svc/eventlog"
)

type windowsExporterService struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func (s *windowsExporterService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				s.cancel()
				break loop
			default:
				logrus.Error(fmt.Sprintf("unexpected control request #%d", c))
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}

func runService(ctx context.Context, log *logrus.Logger) (context.Context, error) {
	isInteractive, err := svc.IsAnInteractiveSession()
	if err != nil {
		return ctx, err
	}

	elog, err := eventlog.Open(serviceName)
	if err != nil {
		return ctx, err
	}

	log.Hooks.Add(el.NewHook(elog))

	serviceCtx, cancel := context.WithCancel(ctx)

	if !isInteractive {
		go func() {
			err = svc.Run(serviceName, &windowsExporterService{ctx: serviceCtx, cancel: cancel})
			if err != nil {
				logrus.WithError(err).Errorf("Failed to start service: %v", err)
			}
		}()
	}

	go func() {
		<-ctx.Done()
		elog.Close()
	}()

	return serviceCtx, nil
}
