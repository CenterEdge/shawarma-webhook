package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/CenterEdge/shawarma-webhook/httpd"
	"github.com/CenterEdge/shawarma-webhook/routes"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

type config struct {
	httpdConf               httpd.Conf
	sideCarConfigFile       string
	shawarmaImage           string
	shawarmaSecretTokenName string
}

// Set on build
var version string

func main() {
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.JSONFormatter{})

	app := cli.NewApp()
	app.Name = "Shawarma Webhook"
	app.Usage = "Kubernetes Mutating Admission Webhook to add the Shawarma sidecar when requested by annotations"
	app.Copyright = "(c) 2019 CenterEdge Software"
	app.Version = version

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "log-level, l",
			Usage:  "Set the log level (panic, fatal, error, warn, info, debug, trace)",
			Value:  "warn",
			EnvVar: "LOG_LEVEL",
		},
		cli.IntFlag{
			Name:   "port, p",
			Usage:  "Set the listening port number",
			Value:  443,
			EnvVar: "WEBHOOK_PORT",
		},
		cli.StringFlag{
			Name:   "cert-file",
			Usage:  "File containing the TLS certificate (PEM encoded)",
			Value:  "./certs/cert.pem",
			EnvVar: "CERT_FILE",
		},
		cli.StringFlag{
			Name:   "key-file",
			Usage:  "File containing the TLS private key (PEM encoded)",
			Value:  "./certs/key.pem",
			EnvVar: "KEY_FILE",
		},
		cli.StringFlag{
			Name:  "config, c",
			Usage: "File containing the sidecar configuration",
			Value: "./sidecar.yaml",
		},
		cli.StringFlag{
			Name:   "shawarma-image",
			Usage:  "Default Docker image",
			Value:  "centeredge/shawarma:0.1.2",
			EnvVar: "SHAWARMA_IMAGE",
		},
		cli.StringFlag{
			Name:   "shawarma-secret-token-name",
			Usage:  "Name of the secret containing the Kubernetes token for Shawarma",
			Value:  "shawarma-token",
			EnvVar: "SHAWARMA_SECRET_TOKEN_NAME",
		},
	}

	app.Before = func(c *cli.Context) error {
		// In case of empty environment variable, pull default here too
		levelString := c.String("log-level")
		if levelString == "" {
			levelString = "warn"
		}

		level, err := log.ParseLevel(levelString)
		if err != nil {
			return err
		}

		log.SetLevel(level)

		return nil
	}

	app.Action = func(c *cli.Context) error {
		conf := readConfig(c)
		simpleServer := httpd.NewSimpleServer(conf.httpdConf)

		var err error
		if err = addRoutes(simpleServer, conf); err != nil {
			return err
		}

		if err = startHTTPServerAndWait(simpleServer); err != nil {
			return err
		}

		log.Infof("Shutting down initiated")
		simpleServer.Shutdown()
		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func addRoutes(simpleServer httpd.SimpleServer, conf config) error {
	mutator, err := routes.NewMutatorController(conf.sideCarConfigFile, conf.shawarmaImage, conf.shawarmaSecretTokenName)
	if err != nil {
		return err
	}

	simpleServer.AddRoute("/mutate", mutator.Mutate)

	health, err := routes.NewHealthController()
	if err != nil {
		return err
	}

	simpleServer.AddRoute("/health", health.Health)

	return nil
}

func readConfig(c *cli.Context) config {
	var conf config

	conf.httpdConf.Port = c.Int("port")
	conf.httpdConf.CertFile = c.String("cert-file")
	conf.httpdConf.KeyFile = c.String("key-file")
	conf.sideCarConfigFile = c.String("config")
	conf.shawarmaImage = c.String("shawarma-image")
	conf.shawarmaSecretTokenName = c.String("shawarma-secret-token-name")

	return conf
}

func startHTTPServerAndWait(simpleServer httpd.SimpleServer) error {
	errC := make(chan error, 1)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	defer func() {
		close(errC)
		close(signalChan)
	}()

	log.Infof("SimpleServer starting to listen in port %v", simpleServer.Port())

	simpleServer.Start(errC)

	// block until an error or signal from os to
	// terminate the process
	var retErr error
	select {
	case err := <-errC:
		retErr = err
	case <-signalChan:
	}

	return retErr
}
