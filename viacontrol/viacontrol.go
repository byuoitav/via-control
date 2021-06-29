package viacontrol

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/byuoitav/kramer-driver"
	"github.com/labstack/echo"
)

type Volume struct {
	Volume int `json:"volume"`
}

type wrappedEchoServer struct {
	*echo.Echo
}

type ViaDevice interface {
	GetVolume(ctx context.Context) (int, error)
	SetVolume(ctx context.Context, volume int) (string, error)
	Reboot(ctx context.Context) error
	Reset(ctx context.Context) error
	GetRoomCode(ctx context.Context) (string, error)
	GetHardwareInfo(ctx context.Context) (kramer.HardwareInfo, error)
	GetStatusOfUsers(ctx context.Context) (kramer.VIAUsers, error)
	SetAlert(ctx context.Context, AMessage string) error
}

type Server interface {
	Serve(lis net.Listener) error
}

func newEchoServer() *echo.Echo {
	e := echo.New()

	return e
}

func wrapEchoServer(e *echo.Echo) Server {
	return &wrappedEchoServer{
		Echo: e,
	}
}

func (e *wrappedEchoServer) Serve(lis net.Listener) error {
	return e.Server.Serve(lis)
}

type CreateVIAFunc func(context.Context, string) (ViaDevice, error)

func CreateVIAServer(create CreateVIAFunc) (Server, error) {
	e := newEchoServer()
	m := &sync.Map{}
	// Magic happens right here
	via := func(ctx context.Context, addr string) (ViaDevice, error) {
		if via, ok := m.Load(addr); ok {
			return via.(ViaDevice), nil
		}

		via, err := create(ctx, addr)
		if err != nil {
			return nil, err
		}

		m.Store(addr, via)
		return via, nil
	}

	addVIARoutes(e, via)

	return wrapEchoServer(e), nil

}
func addVIARoutes(e *echo.Echo, create CreateVIAFunc) {
	// volume
	e.GET("/:address/volume/level", func(c echo.Context) error {
		addr := c.Param("address")

		if len(addr) == 0 {
			return c.String(http.StatusBadRequest, "must include the address of the VIA")
		}

		d, err := create(c.Request().Context(), addr)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		volume, err := d.GetVolume(c.Request().Context())
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		return c.JSON(http.StatusOK, volume)
	})

	e.GET("/:address/volume/set/:volvalue", func(c echo.Context) error {
		address := c.Param("address")
		value := c.Param("volvalue")
		fmt.Printf("Value passed by SetViaVolume is %v\n", value)

		volume, err := strconv.Atoi(value)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err.Error())
		} else if volume > 100 || volume < 1 {
			fmt.Errorf("Volume command error - volume value %s is outside the bounds of 1-100", value)
			return c.JSON(http.StatusBadRequest, "Error: volume must be a value from 1 to 100!")
		}

		fmt.Printf("Setting volume for %s to %v...\n", address, volume)

		d, err := create(c.Request().Context(), address)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		response, err := d.SetVolume(c.Request().Context(), volume)

		if err != nil {
			return c.JSON(http.StatusBadRequest, "An error has occured while setting volume")
		}
		fmt.Printf("Success: %s\n", response)

		return c.JSON(http.StatusOK, Volume{Volume: volume})

	})
	// VIA Reset and Rebooting of Endpoints
	e.GET("/:address/reset", func(c echo.Context) error {

		address := c.Param("address")

		d, err := create(c.Request().Context(), address)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		err = d.Reset(c.Request().Context())
		if err != nil {
			fmt.Errorf("There was a problem: %v", err.Error())
			return c.JSON(http.StatusInternalServerError, err.Error())
		}

		return c.JSON(http.StatusOK, "Success")
	})

	e.GET("/:address/reboot", func(c echo.Context) error {
		address := c.Param("address")

		d, err := create(c.Request().Context(), address)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		err = d.Reboot(c.Request().Context())
		if err != nil {
			fmt.Errorf("There was a problem: %v\n", err.Error())
			return c.JSON(http.StatusInternalServerError, err.Error())
		}

		return c.JSON(http.StatusOK, "Success")
	})

	// Informational Endpoints
	e.GET("/:address/hardware", func(c echo.Context) error {
		address := c.Param("address")

		d, err := create(c.Request().Context(), address)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		hardware, err := d.GetHardwareInfo(c.Request().Context())
		if err != nil {
			fmt.Errorf("Error getting hardware status: %s\n", err.Error())
			return c.JSON(http.StatusInternalServerError, err.Error())
		}

		return c.JSON(http.StatusOK, hardware)
	})
	// Second Hardware Endpoint
	e.GET("/:address/info", func(c echo.Context) error {
		address := c.Param("address")

		d, err := create(c.Request().Context(), address)
		if err != nil {
			fmt.Errorf("Error Getting full info: %s\n", err.Error())
			return c.JSON(http.StatusInternalServerError, err.Error())
		}

		info, err := d.GetInfo(c.Request().Context())
		if err != nil {
			fmt.Errorf("Error Getting Info: %s\n", err.Error())
			return c.JSON(http.StatusInternalServerError, err.Error())
		}

		return c.JSON(http.StatusOK, info)
	})

	/*e.GET("/:address/active", func(c echo.Context) error {
		address := c.Param("address")
		d, err := create(c.Request().Context(), address)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		signal, err := d.GetActiveSignal(c.Param("address"))
		if err != nil {
			log.L.Errorf("Failed to retrieve VIA active signal: %s", err.Error())
			return c.JSON(http.StatusInternalServerError, err)
		}

		return c.JSON(http.StatusOK, signal)
	})
	*/
	// Get the current room code for a room
	e.GET("/:address/roomcode", func(c echo.Context) error {
		address := c.Param("address")

		d, err := create(c.Request().Context(), address)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		code, err := d.GetRoomCode(c.Request().Context())
		if err != nil {
			fmt.Errorf("Failed to retrieve VIA room code: %s", err.Error())
			return c.JSON(http.StatusInternalServerError, err)
		}
		return c.JSON(http.StatusOK, code)

	})

	e.GET("/:address/users/status", func(c echo.Context) error {
		address := c.Param("address")

		d, err := create(c.Request().Context(), address)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		userlist, err := d.GetStatusOfUsers(c.Request().Context())
		if err != nil {
			fmt.Errorf("Failed to retrieve current user list: %s", err.Error())
			return c.JSON(http.StatusInternalServerError, err)
		}

		return c.JSON(http.StatusOK, userlist)
	})
	// Send an alert to a VIA
	e.GET("/:address/alert/message/:message", func(c echo.Context) error {
		address := c.Param("address")
		message := c.Param("message")

		d, err := create(c.Request().Context(), address)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, err)
		}

		err = d.SetAlert(c.Request().Context(), message)
		if err != nil {
			fmt.Errorf("Failed to send alert to %s: %s", address, err.Error())
			return c.JSON(http.StatusInternalServerError, err)
		}

		return c.JSON(http.StatusOK, "Success")
	})
}
