package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/byuoitav/common/db"
	"github.com/byuoitav/common/structs"
	"github.com/byuoitav/kramer-driver/kramer"
	"github.com/byuoitav/via-control/monitor"
	"github.com/byuoitav/via-control/viacontrol"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

/* global variable declaration */
// Changed: lowercase vars
var name string
var deviceList []structs.Device

func init() {

	if len(os.Getenv("ROOM_SYSTEM")) == 0 {
		fmt.Printf("System is not tied to a specific room. Will not start via monitoring\n")
		return
	}

	name = os.Getenv("SYSTEM_ID")
	var err error
	fmt.Printf("Gathering information for %s from database\n", name)

	s := strings.Split(name, "-")
	sa := s[0:2]
	room := strings.Join(sa, "-")

	fmt.Printf("Waiting for database . . . .\n")
	for {
		// Pull room information from db
		state, err := db.GetDB().GetStatus()
		fmt.Printf("%v\n", state)
		//+deploy not_requried
		if (err != nil || state != "completed") && !(len(os.Getenv("DEV_ROUTER")) > 0 || len(os.Getenv("STOP_REPLICATION")) > 0) {
			fmt.Printf("Database replication in state %v. Retrying in 5 seconds.\n", state)
			time.Sleep(5 * time.Second)
			continue
		}
		fmt.Printf("Database replication state: %v\n", state)

		devices, err := db.GetDB().GetDevicesByRoomAndRole(room, "EventRouter")
		if err != nil {
			fmt.Printf("Connecting to the Configuration DB failed, retrying in 5 seconds.\n")
			time.Sleep(5 * time.Second)
			continue
		}

		if len(devices) == 0 {
			//there's a chance that there ARE routers in the room, but the initial database replication is occuring.
			//we're good, keep going
			state, err := db.GetDB().GetStatus()
			if (err != nil || state != "completed") && !(len(os.Getenv("STOP_REPLICATION")) > 0) {
				fmt.Printf("Database replication in state %v. Retrying in 5 seconds.\n", state)
				time.Sleep(5 * time.Second)
				continue
			}
		}
		fmt.Printf("Connection to the Configuration DB established.\n")
		break
	}
	deviceList, err = db.GetDB().GetDevicesByRoomAndType(room, "via-connect-pro")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

func main() {
	var (
		port     int
		username string
		password string
		logLevel int8
	)

	pflag.IntVarP(&port, "port", "P", 8014, "port to run the microservice on")
	pflag.StringVarP(&username, "username", "u", "", "username for device")
	pflag.StringVarP(&password, "password", "p", "", "password for device")
	pflag.Int8VarP(&logLevel, "log-level", "L", 0, "Level to log at. Provided by zap logger: https://godoc.org/go.uber.org/zap/zapcore")
	pflag.Parse()

	// Build out the Logger
	config := zap.Config{
		Level:       zap.NewAtomicLevelAt(zapcore.Level(logLevel)),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding: "json",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "@",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}

	plain, err := config.Build()
	if err != nil {
		fmt.Printf("unable to build logger you foolish mortal: %s", err)
		os.Exit(1)
	}

	sugared := plain.Sugar()

	addr := fmt.Sprintf(":%d", port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Printf("failed to start server: %s\n", err)
		os.Exit(1)
	}
	sugared.Infof("This is the addr value: ", addr)
	// import driver library for ViaControl -
	createVia := func(ctx context.Context, addr string) (viacontrol.ViaDevice, error) {
		return &kramer.Via{
			Address:  addr,
			Username: username,
			Password: password,
			Logger:   sugared,
		}, nil
	}
	// Building ViaStruct for use with the ViaMonitor -
	sugared.Infof("Building Via struct for controller.....")
	v := &kramer.Via{
		Address:  addr,
		Username: username,
		Password: password,
		Logger:   sugared,
	}

	var re = regexp.MustCompile(`-CP1$`)
	test := re.MatchString(name)
	var ctx context.Context

	//start the persistent VIA monitoring connection if the Controller is CP1
	if test == true && len(os.Getenv("ROOM_SYSTEM")) > 0 {
		for _, device := range deviceList {
			go monitor.StartMonitoring(ctx, device, v)
		}
	}

	// create server for ViaControl -
	server, err := viacontrol.CreateVIAServer(createVia)
	if err != nil {
		fmt.Printf("Error while trying to create DSP Server: %s\n", err)
		os.Exit(1)
	}

	if err = server.Serve(lis); err != nil {
		fmt.Printf("failed to listen: %s\n", err)
		os.Exit(1)
	}

}
