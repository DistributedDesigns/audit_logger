package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

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

	rmqConn *amqp.Connection
	db      *sql.DB
)

const (
	// Named RMQ queues / exchanges
	auditEventQ      = "audit_event"
	dumplogQ         = "dumplog"
	quoteBroadcastEx = "quote_broadcast"
)

func main() {
	kingpin.Parse()

	initConsoleLogging()

	loadConfig()

	initRMQ()
	defer rmqConn.Close()

	initDB()
	defer db.Close()

	initLogDirectory()
	auditlogFile := createNewLogfile()
	defer auditlogFile.Close()

	// Write our logfile header
	auditlogFile.WriteString("<?xml version=\"1.0\"?>\n<log>")

	events := make(chan string)
	dump := make(chan struct{})
	done := make(chan struct{})

	go quoteCatcher(events, done)
	go auditEventCatcher(events, done)
	go dumplogWatcher(dump, done)
	go func() {
		// write events as they're caught
		for event := range events {
			auditlogFile.WriteString(event)
		}
	}()

	// run go-routines until dump is sent
	<-dump

	// Halts all go-routines
	close(done)

	// Write the footer to the logfile before we close it on exit
	auditlogFile.WriteString("\n</log>\n")
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
		`%{time:15:04:05.000} %{color}▶ %{level:8s}%{color:reset} %{id:03d} %{message} %{shortfile}`,
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
		Host string
		Port int
		User string
		Pass string
	}

	DB struct {
		Host     string
		Port     int
		User     string
		Password string
		AuditDB  string `yaml:"auditdb"`
	} `yaml:"database"`
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

func initRMQ() {
	rabbitAddress := fmt.Sprintf("amqp://%s:%s@%s:%d",
		config.Rabbit.User, config.Rabbit.Pass,
		config.Rabbit.Host, config.Rabbit.Port,
	)

	var err error
	rmqConn, err = amqp.Dial(rabbitAddress)
	failOnError(err, "Failed to rmqConnect to RabbitMQ")
	// closed in main()

	ch, err := rmqConn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	// Make sure all of the expected RabbitMQ exchanges and queues
	// exist before we start using them.
	// Recieve audit events
	_, err = ch.QueueDeclare(
		auditEventQ, // name
		true,        // durable
		false,       // delete when unused
		false,       // exclusive
		false,       // no wait
		nil,         // arguments
	)
	failOnError(err, "Failed to declare a queue")

	// Recieve dumplog commands
	_, err = ch.QueueDeclare(
		dumplogQ, // name
		true,     // durable
		false,    // delete when unused
		false,    // exclusive
		false,    // no wait
		nil,      // arguments
	)
	failOnError(err, "Failed to declare a queue")

	// Catch quote updates
	err = ch.ExchangeDeclare(
		quoteBroadcastEx,   // name
		amqp.ExchangeTopic, // type
		true,               // durable
		false,              // auto-deleted
		false,              // internal
		false,              // no-wait
		nil,                // args
	)
	failOnError(err, "Failed to declare an exchange")
}

func initDB() {
	dbConnection := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		config.DB.User, config.DB.Password, config.DB.Host, config.DB.Port, config.DB.AuditDB,
	)

	var err error
	db, err = sql.Open("mysql", dbConnection)
	failOnError(err, "Could not open DB connection")
	// closed in main()

	// TODO: connection pooling and config???

	// Test to see if connection works
	err = db.Ping()
	failOnError(err, "Could not ping DB")
}

func createNewLogfile() *os.File {
	// Create a new file based on the current time
	now := time.Now()
	auditlogFileName := fmt.Sprintf("%s/%d-%02d-%02dT%02d%02d%02d.xml",
		*logfileDir, now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second(),
	)

	// Open the file for writing
	auditlogFile, err := os.Create(auditlogFileName)
	failOnError(err, "Could not create logfile")

	consoleLog.Info(" [-] Writing log to", auditlogFileName)

	return auditlogFile
}
