package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/pflag"
	//"github.com/byuoitav/common"
	"github.com/byuoitav/common/db"
	"github.com/byuoitav/common/log"
	"github.com/byuoitav/common/structs"
	"github.com/byuoitav/kramer-driver/via"
	"github.com/byuoitav/via-control/monitor"
	"github.com/byuoitav/via-control/viacontrol"
)

/* global variable declaration */
// Changed: lowercase vars
var name string
var deviceList []structs.Device

func init() {

	if len(os.Getenv("ROOM_SYSTEM")) == 0 {
		log.L.Debugf("System is not tied to a specific room. Will not start via monitoring")
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
		log.L.Debugf("%v\n", state)
		//+deploy not_requried
		if (err != nil || state != "completed") && !(len(os.Getenv("DEV_ROUTER")) > 0 || len(os.Getenv("STOP_REPLICATION")) > 0) {
			log.L.Debugf("Database replication in state %v. Retrying in 5 seconds.", state)
			time.Sleep(5 * time.Second)
			continue
		}
		log.L.Debugf("Database replication state: %v", state)

		devices, err := db.GetDB().GetDevicesByRoomAndRole(room, "EventRouter")
		if err != nil {
			log.L.Debugf("Connecting to the Configuration DB failed, retrying in 5 seconds.")
			time.Sleep(5 * time.Second)
			continue
		}

		if len(devices) == 0 {
			//there's a chance that there ARE routers in the room, but the initial database replication is occuring.
			//we're good, keep going
			state, err := db.GetDB().GetStatus()
			if (err != nil || state != "completed") && !(len(os.Getenv("STOP_REPLICATION")) > 0) {
				log.L.Debugf("Database replication in state %v. Retrying in 5 seconds.", state)
				time.Sleep(5 * time.Second)
				continue
			}
		}
		log.L.Debugf("Connection to the Configuration DB established.")
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
	)

	pflag.IntVarP(&port, "port", "P", 8014, "port to run the microservice on")
	pflag.StringVarP(&username, "username", "u", "", "username for device")
	pflag.StringVarP(&password, "password", "p", "", "password for device")

	pflag.Parse()

	addr := fmt.Sprintf(":%d", port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Printf("failed to start server: %s\n", err)
		os.Exit(1)
	}
	log.L.Info("This is the addr value: ", addr)
	// import driver library
	createVia := func(ctx context.Context, addr string) (viacontrol.ViaDevice, error) {
		return &via.VIA{
			Address:  addr,
			Username: username,
			Password: password,
		}, nil
	}

	var re = regexp.MustCompile(`-CP3$`)
	test := re.MatchString(name)
	var ctx context.Context
	//start the VIA monitoring connection if the Controller is CP1
	if test == true && len(os.Getenv("ROOM_SYSTEM")) > 0 {
		for _, device := range deviceList {
			go monitor.StartMonitoring(ctx, device, username, password)
		}
	}

	// create server
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
