package monitor

import (
	"bufio"
	"context"
	"fmt"
	//	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/byuoitav/central-event-system/hub/base"
	ces "github.com/byuoitav/central-event-system/messenger"
	//	"github.com/byuoitav/common/log"
	"github.com/byuoitav/common/nerr"
	"github.com/byuoitav/common/structs"
	"github.com/byuoitav/common/v2/events"
	"github.com/byuoitav/kramer-driver/kramer"
)

const (
	// Intervals to wait between retry attempts
	reconnInterval = 10 * time.Second

	// Ping Internal (in milliseconds, because it cares)
	pingInterval = 60000
)

var (
	pihost     string
	hostname   string
	buildingID string
	room       string
)

func init() {
	var err error
	if len(os.Getenv("ROOM_SYSTEM")) == 0 {
		return

	}

	pihost = os.Getenv("SYSTEM_ID")
	if len(pihost) == 0 {
		fmt.Errorf("SYSTEM_ID not set.\n")
	}

	hostname, err = os.Hostname()
	if err != nil {
		hostname = pihost
	}

	split := strings.Split(pihost, "-")
	buildingID = split[0]
	room = split[1]

}

type message struct {
	EventType string
	Action    string
	User      string
	State     string
}

// Retry connection if connection has failed
func retryViaConnection(ctx context.Context, device structs.Device, pconn *kramer.PersistentViaConnection, event events.Event, v *kramer.Via) {
	v.Infof("[retry] Retrying Connection to VIA")
	v.Address = device.Address
	pconn, err := v.PersistConnection(ctx)
	for err != nil {
		v.Errorf("Retry Failed, Trying again in 10 seconds")
		time.Sleep(reconnInterval)
		//		pconn, err = (*VIA).PersistConnection(addr, viaUser, viaPass)
	}

	go readPump(ctx, device, pconn, event, v)
	go writePump(ctx, device, pconn, v)
}

// Read events and send them to console
func readPump(ctx context.Context, device structs.Device, pconn *kramer.PersistentViaConnection, event events.Event, v *kramer.Via) {
	// defer closing connection
	defer func(device structs.Device) {
		(pconn.Conn).Close()
		v.Errorf("Connection to VIA %v is dying.", device.Address)
		v.Infof("Trying to reconnect........")
		//retry connection to VIA device
		retryViaConnection(ctx, device, pconn, event, v)
	}(device)
	timeoutDuration := 300 * time.Second

	for {
		var m message
		//set deadline for reads - keep the connection alive during that time
		(pconn.Conn).SetReadDeadline(time.Now().Add(timeoutDuration))
		//start reader to read into buffer
		reader := bufio.NewReader(pconn.Reader)
		r, err := reader.ReadBytes('\x0D')
		if err != nil {
			err = fmt.Errorf("error reading from system: %s", err.Error())
			v.Errorf(err.Error())
			return
		}
		//Buffer = append(Buffer, tmp[:r]...)

		str := fmt.Sprintf("%s", r)
		trim := strings.TrimSpace(str)
		Out := strings.Split(trim, "|")
		switch {
		// How many people logged in
		case Out[0] == "PList" && Out[2] == "cnt":
			m.EventType = "current-user-count"
			m.Action = "login-count"
			m.User = Out[2]
			i, err := strconv.Atoi(Out[3])
			if err != nil {
				fmt.Printf("Error: %v\n", err.Error())
			}

			i--
			loggedinCount := strconv.Itoa(i)
			fmt.Printf("The number of people logged in is %v\n", loggedinCount)
			m.State = loggedinCount

		// Who just logged in
		case Out[0] == "PList" && !(Out[2] == "cnt"):
			m.EventType = "user-login-logout"
			if Out[2] == "1" {
				m.Action = "login"
				fmt.Printf("%v - Login\n", Out[2])
			} else if Out[2] == "0" {
				m.Action = "logout"
				fmt.Printf("%v - Logout\n", Out[2])
			}
			m.User = Out[2]
			m.State = m.Action
		// Started or stopped media
		case Out[0] == "MediaStatus":
			m.EventType = Out[0]
			if Out[2] == "1" {
				m.Action = "media-started"
				fmt.Printf("Media Started\n")
			} else if Out[2] == "0" {
				m.Action = "media-stopped"
				fmt.Printf("Media Stopped\n")
			}
			m.User = ""
			m.State = m.Action
		// Started or Stopped Presenting
		case Out[0] == "DisplayStatus":
			m.EventType = "presenting"
			if Out[3] == "1" {
				m.Action = "presentation-started"
				fmt.Printf("%v - Presentation Started\n", Out[2])
			} else if Out[3] == "0" {
				m.Action = "presentation-stopped"
				fmt.Printf("%v - Presentation Stopped\n", Out[2])
			}
			m.User = Out[2]
			m.State = m.Action

			//QueryPresentationNumber(event, messenger().SendEvent)

		// Stop our friend ping from sending on because we don't like ping, He's not really our friend
		default:
			continue
		}

		event.Timestamp = time.Now()
		event.Key = m.EventType
		event.Value = m.State
		event.User = m.User

		// changed: add event stuff
		messenger().SendEvent(event)
	}
}

func writePump(ctx context.Context, device structs.Device, pconn *kramer.PersistentViaConnection, v *kramer.Via) {
	// defer closing connection
	defer func(device structs.Device) {
		(pconn.Conn).Close()
		v.Errorf("Error on write pump for %v. Write pump closing.", device.Address)
	}(device)
	ticker := time.NewTicker(pingInterval * time.Millisecond)
	// Once the pingInterval is reached, execute the ping -
	// On Error, return and execute deferred to close the connection
	for range ticker.C {
		err := v.Ping(pconn)
		if err != nil {
			v.Errorf("Ping Failed Error: %v", err)
			return
		}
	}
}

// StartMonitoring service for each VIA in a room
func StartMonitoring(ctx context.Context, device structs.Device, v *kramer.Via) *kramer.PersistentViaConnection {
	v.Debugf("Building Connection and starting read buffer for %s\n", device.Address)
	v.Address = device.Address
	pconn, err := v.PersistConnection(ctx)
	for err != nil {
		v.Errorf("Retry Failed, Trying again in 10 seconds")
		time.Sleep(reconnInterval)
		pconn, err = v.PersistConnection(ctx)
	}

	// start event node
	_ = messenger()

	roomID := fmt.Sprintf("%s-%s", buildingID, room)

	// build base event to send along with each event
	event := events.Event{
		GeneratingSystem: pihost,
		AffectedRoom:     events.GenerateBasicRoomInfo(roomID),
		TargetDevice:     events.GenerateBasicDeviceInfo(device.ID),
		User:             hostname,
	}

	event.AddToTags(events.DetailState, events.AutoGenerated, events.Via)

	go readPump(ctx, device, pconn, event, v)
	go writePump(ctx, device, pconn, v)
	return pconn
}

var once sync.Once
var msg *ces.Messenger

func messenger() *ces.Messenger {
	once.Do(func() {
		hub := os.Getenv("HUB_ADDRESS")
		if len(hub) == 0 {
			fmt.Errorf("HUB_ADDRESS is not set.")
		}

		var nerr *nerr.E
		msg, nerr = ces.BuildMessenger(hub, base.Messenger, 1000)
		if nerr != nil {
			fmt.Errorf("failed to build the messenger: %s", nerr.String())
			return
		}
	})

	return msg
}
