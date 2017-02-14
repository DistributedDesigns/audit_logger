package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/op/go-logging"
	"github.com/streadway/amqp"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
	yaml "gopkg.in/yaml.v2"
)

// Globals
var (
	logLevels = []string{"CRITICAL", "ERROR", "WARNING", "NOTICE", "INFO", "DEBUG"}
	logLevel  = kingpin.
			Flag("log-level", fmt.Sprintf("Minimum level for logging to the console. Must be one of: %s", strings.Join(logLevels, ", "))).
			Default("WARNING").
			Short('l').
			Enum(logLevels...)
	serviceID = kingpin.
			Flag("service-id", "Logging name for the service").
			Default("audit").
			Short('s').
			String()
	configFile = kingpin.
			Flag("config", "YAML file with service config").
			Default("./config/dev.yaml").
			Short('c').
			ExistingFile()
	logfileDir = kingpin.
			Flag("log-directory", "Directory that will hold audit logs").
			Default("logs").
			Short('d').
			String()

	consoleLog = logging.MustGetLogger("console")

	rmqConn      *amqp.Connection
	auditlogFile *os.File
)

func main() {
	kingpin.Parse()
	initConsoleLogging()
	loadConfig()
	initLogDirectory()

	// Create a new file based on the current time
	now := time.Now()
	auditlogFileName := fmt.Sprintf("%s/%d-%02d-%02dT%02d%02d%02d.xml",
		*logfileDir, now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second(),
	)

	// Open the file for writing
	auditlogFile, err := os.Create(auditlogFileName)
	failOnError(err, "Could not create logfile")
	defer func() {
		// Write the footer and close file
		auditlogFile.WriteString("</log>\n")
		auditlogFile.Close()
	}()

	// Write the logfile header
	auditlogFile.WriteString("<?xml version=\"1.0\"?>\n<log>\n")
}

func failOnError(err error, msg string) {
	if err != nil {
		consoleLog.Fatalf("%s: %s", msg, err)
	}
}

func initConsoleLogging() {

	// Create a default backend
	consoleBackend := logging.NewLogBackend(os.Stdout, "", 0)

	// Add output formatting
	var consoleFormat = logging.MustStringFormatter(
		`%{time:15:04:05.000} %{color}▶ %{level:8s}%{color:reset} %{id:03d} %{message}  %{shortfile}`,
	)
	consoleBackendFormatted := logging.NewBackendFormatter(consoleBackend, consoleFormat)

	// Add leveled logging
	level, err := logging.LogLevel(*logLevel)
	if err != nil {
		fmt.Println("Bad log level. Using default level of ERROR")
	}
	consoleBackendFormattedAndLeveled := logging.AddModuleLevel(consoleBackendFormatted)
	consoleBackendFormattedAndLeveled.SetLevel(level, "")

	// Attach the backend
	logging.SetBackend(consoleBackendFormattedAndLeveled)
}

// Holds values from <config>.yaml.
// 'PascalCase' values come from 'pascalcase' in x.yaml
var config struct {
	Rabbit struct {
		Host   string
		Port   int
		User   string
		Pass   string
		Queues struct {
			Audit   string
			Dumplog string
		}
		Exchanges struct {
			QuoteBroadcast string `yaml:"quote broadcast"`
		}
	}
}

func loadConfig() {
	// Load the yaml file
	data, err := ioutil.ReadFile(*configFile)
	failOnError(err, "Could not read file")

	err = yaml.Unmarshal(data, &config)
	failOnError(err, "Could not unmarshal config")
}

func initLogDirectory() {
	// Create the output directory, if we need to
	if _, err := os.Stat(*logfileDir); os.IsNotExist(err) {
		consoleLog.Debugf("Creating log directory at ./%s", *logfileDir)
		if dirErr := os.Mkdir(*logfileDir, 0755); dirErr != nil {
			// Don't bother running if we can't generate a log file.
			failOnError(dirErr, "Couldn't create log directory")
		}
	}

}
