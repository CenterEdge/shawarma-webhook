package main

import (
	"context"
	"os"

	"github.com/CenterEdge/shawarma-webhook/httpd"
	"github.com/CenterEdge/shawarma-webhook/routes"
	"github.com/CenterEdge/shawarma-webhook/webhook"
	cli "github.com/urfave/cli/v3"
	"go.uber.org/zap"
)

type config struct {
	httpdConf               httpd.Conf
	sideCarConfigFile       string
	shawarmaImage           string
	shawarmaServiceAcctName string
	shawarmaSecretTokenName string
	nativeSidecars          bool
}

// Set on build
var version string

func main() {
	logLevel := zap.NewAtomicLevelAt(zap.WarnLevel)
	logConfig := zap.NewProductionConfig()
	logConfig.Level = logLevel

	logger := zap.Must(logConfig.Build())
	defer logger.Sync() // flushes buffer, if any

	app := cli.Command{
		Name:            "Shawarma Webhook",
		Usage:           "Kubernetes Mutating Admission Webhook to add the Shawarma sidecar when requested by annotations",
		Copyright:       "(c) 2019-2025 CenterEdge Software",
		Version:         version,
		HideHelpCommand: true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "log-level",
				Aliases: []string{"l"},
				Usage:   "Set the log level (panic, fatal, error, warn, info, debug, trace)",
				Value:   "warn",
				Sources: cli.EnvVars("LOG_LEVEL"),
			},
			&cli.Uint16Flag{
				Name:    "port",
				Aliases: []string{"p"},
				Usage:   "Set the listening port number",
				Value:   8443,
				Sources: cli.EnvVars("WEBHOOK_PORT"),
			},
			&cli.StringFlag{
				Name:    "cert-file",
				Usage:   "File containing the TLS certificate (PEM encoded)",
				Value:   "./certs/tls.crt",
				Sources: cli.EnvVars("CERT_FILE"),
			},
			&cli.StringFlag{
				Name:    "key-file",
				Usage:   "File containing the TLS private key (PEM encoded)",
				Value:   "./certs/tls.key",
				Sources: cli.EnvVars("KEY_FILE"),
			},
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "File containing the sidecar configuration",
				Value:   "./sidecar.yaml",
			},
			&cli.StringFlag{
				Name:    "shawarma-image",
				Usage:   "Default Docker image",
				Value:   "centeredge/shawarma:2.0.0-beta001",
				Sources: cli.EnvVars("SHAWARMA_IMAGE"),
			},
			&cli.BoolFlag{
				Name:    "native-sidecars",
				Usage:   "Use Kubernetes (>=1.29) native sidecars",
				Value:   true,
				Sources: cli.EnvVars("SHAWARMA_NATIVE_SIDECARS"),
			},
			&cli.StringFlag{
				Name:    "shawarma-service-acct-name",
				Usage:   "Name of the service account which should be used for sidecars (requires a legacy token secret linked to the service account)",
				Value:   "",
				Sources: cli.EnvVars("SHAWARMA_SERVICE_ACCT_NAME"),
			},
			&cli.StringFlag{
				Name:    "shawarma-secret-token-name",
				Usage:   "Name of the secret containing the Kubernetes token for Shawarma, overrides shawarma-service-acct-name",
				Value:   "",
				Sources: cli.EnvVars("SHAWARMA_SECRET_TOKEN_NAME"),
			},
		},
		Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
			// In case of empty environment variable, pull default here too
			levelString := c.String("log-level")
			if levelString != "" {
				if level, err := zap.ParseAtomicLevel(levelString); err == nil {
					logLevel.SetLevel(level.Level())
				} else {
					return ctx, err
				}
			}

			return ctx, nil
		},
	}

	app.Action = func(ctx context.Context, c *cli.Command) error {
		conf := readConfig(c, logger)

		if conf.shawarmaServiceAcctName != "" {
			// If using a service account token, startup the monitor for service accounts
			err := webhook.InitializeServiceAcctMonitor()
			if err != nil {
				logger.Warn("Error initializing service account monitor",
					zap.Error(err))
			}
		}

		simpleServer := httpd.NewSimpleServer(conf.httpdConf)

		webhook.Init()

		var (
			mutator routes.MutatorController
			err     error
		)

		if mutator, err = addRoutes(simpleServer, conf); err != nil {
			return err
		}

		if err = simpleServer.StartAndWait(); err != nil {
			return err
		}

		logger.Info("Shutdown initiated")
		simpleServer.Shutdown()
		mutator.Shutdown()
		return nil
	}

	err := app.Run(context.Background(), os.Args)
	if err != nil {
		logger.Fatal("Fatal error",
			zap.Error(err))
	}
}

func addRoutes(simpleServer httpd.SimpleServer, conf *config) (routes.MutatorController, error) {
	mutator, err := routes.NewMutatorController(&webhook.MutatorConfig{
		SideCarConfigFile:       conf.sideCarConfigFile,
		ShawarmaImage:           conf.shawarmaImage,
		NativeSidecars:          conf.nativeSidecars,
		ShawarmaServiceAcctName: conf.shawarmaServiceAcctName,
		ShawarmaSecretTokenName: conf.shawarmaSecretTokenName,
		Logger:                  conf.httpdConf.Logger,
	})
	if err != nil {
		return nil, err
	}

	simpleServer.AddRoute("/mutate", mutator.Mutate)

	health, err := routes.NewHealthController(conf.httpdConf.Logger)
	if err != nil {
		return nil, err
	}

	simpleServer.AddRoute("/health", health.Health)

	return mutator, nil
}

func readConfig(c *cli.Command, logger *zap.Logger) *config {
	conf := config{
		httpdConf: httpd.Conf{
			Port:     c.Uint16("port"),
			CertFile: c.String("cert-file"),
			KeyFile:  c.String("key-file"),
			Logger:   logger,
		},
		sideCarConfigFile:       c.String("config"),
		shawarmaImage:           c.String("shawarma-image"),
		shawarmaServiceAcctName: c.String("shawarma-service-acct-name"),
		shawarmaSecretTokenName: c.String("shawarma-secret-token-name"),
		nativeSidecars:          c.Bool("native-sidecars"),
	}

	return &conf
}
