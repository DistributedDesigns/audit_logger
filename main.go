package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
	_ "github.com/lib/pq"
	"github.com/op/go-logging"
	"github.com/streadway/amqp"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
	yaml "gopkg.in/yaml.v2"
)

// Globals
var (
	dump = make(chan struct{})
	done = make(chan struct{})

	consoleLog = logging.MustGetLogger("console")

	auditEventKey string
	dbConnAddr    string
	redisPool     *redis.Pool
	rmqConn       *amqp.Connection
)

const (
	// Named RMQ queues / exchanges
	auditEventQ      = "audit_event"
	dumplogQ         = "dumplog"
	quoteBroadcastEx = "quote_broadcast"
)

func main() {
	loadConfig()

	initConsoleLogging()

	initRMQ()
	defer rmqConn.Close()

	initDB()

	initRedis()

	initLogDirectory()

	// Start worker pools
	for id := 1; id <= config.Workers.RMQ; id++ {
		go auditEventWorker(id)
	}

	for id := 1; id <= config.Workers.DB.Insert; id++ {
		go logInsertWorker(id)
	}

	for id := 1; id <= config.Workers.DB.Dumplog; id++ {
		go dumplogWatcher(id)
	}

	go quoteCatcher()

	//TODO: catch OS interrupt and shut down nicely
	<-done
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
		`%{time:15:04:05.000} %{color}▶ %{level:8s}%{color:reset} %{id:03d} %{message} (%{shortfile})`,
	)
	consoleBackendFormatted := logging.NewBackendFormatter(consoleBackend, consoleFormat)

	// Add leveled logging
	level, err := logging.LogLevel(config.env.logLevel)
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
	env struct {
		logLevel   string
		serviceID  string
		configFile string
		logFileDir string
	}

	// Yaml stuff, has to be Exported
	Rabbit struct {
		Host string
		Port int
		User string
		Pass string
	}

	DB struct {
		Host     string
		Port     int
		SSLmode  string `yaml:"sslmode"`
		User     string
		Password string
		AuditDB  string `yaml:"auditdb"`
	} `yaml:"database"`

	Redis struct {
		Host        string
		Port        int
		MaxIdle     int    `yaml:"max_idle_connections"`
		MaxActive   int    `yaml:"max_active_connections"`
		IdleTimeout int    `yaml:"idle_timeout"`
		KeyPrefix   string `yaml:"key_prefix"`
	}

	Workers struct {
		RMQ int `yaml:"rmq"`
		DB  struct {
			Insert  int
			Dumplog int
		} `yaml:"db"`
	}
}

func loadConfig() {
	app := kingpin.New("audit_logger", "Store records for safe keeping")

	var logLevels = []string{"CRITICAL", "ERROR", "WARNING", "NOTICE", "INFO", "DEBUG"}

	app.Flag("log-level", fmt.Sprintf("Minimum level for logging to the console. Must be one of: %s", strings.Join(logLevels, ", "))).
		Default("WARNING").
		Short('l').
		EnumVar(&config.env.logLevel, logLevels...)

	app.Flag("service-id", "Logging name for the service").
		Default("audit").
		Short('s').
		StringVar(&config.env.serviceID)

	app.Flag("config", "YAML file with service config").
		Default("./config/dev.yaml").
		Short('c').
		ExistingFileVar(&config.env.configFile)

	app.Flag("log-directory", "Directory that will hold audit logs").
		Default("logs").
		Short('d').
		StringVar(&config.env.logFileDir)

	kingpin.MustParse(app.Parse(os.Args[1:]))

	// Load the yaml file
	data, err := ioutil.ReadFile(config.env.configFile)
	if err != nil {
		panic(err)
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		panic(err)
	}

	// Set globals
	auditEventKey = config.Redis.KeyPrefix + ":pendingEvents"
}

func initLogDirectory() {
	// Create the output directory, if we need to
	logDir := config.env.logFileDir
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		consoleLog.Debugf("Creating log directory at ./%s", logDir)
		if dirErr := os.Mkdir(logDir, 0755); dirErr != nil {
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
	dbConnAddr = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		config.DB.User, config.DB.Password, config.DB.Host, config.DB.Port,
		config.DB.AuditDB, config.DB.SSLmode,
	)

	// Get a one-time connection for testing
	db, err := sql.Open("postgres", dbConnAddr)
	failOnError(err, "Could not open DB connection")
	defer db.Close()

	// TODO: connection pooling and config???

	// Test to see if connection works
	err = db.Ping()
	failOnError(err, "Could not ping DB")
}

func initRedis() {
	redisAddress := fmt.Sprintf("%s:%d", config.Redis.Host, config.Redis.Port)

	redisPool = &redis.Pool{
		MaxIdle:     config.Redis.MaxIdle,
		MaxActive:   config.Redis.MaxActive,
		IdleTimeout: time.Second * time.Duration(config.Redis.IdleTimeout),
		Dial:        func() (redis.Conn, error) { return redis.Dial("tcp", redisAddress) },
	}

	// Test if we can talk to redis
	conn := redisPool.Get()
	defer conn.Close()

	_, err := conn.Do("PING")
	failOnError(err, "Could not establish connection with Redis")
}
