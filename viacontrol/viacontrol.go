package viacontrol

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/byuoitav/common/log"
	"github.com/byuoitav/common/status"
	"github.com/byuoitav/common/structs"
	"github.com/labstack/echo"
)

type wrappedEchoServer struct {
	*echo.Echo
}

type ViaDevice interface {
	GetVolume(ctx context.Context) (int, error)
	SetVolume(ctx context.Context, volume string) (string, error)
	RebootVIA(ctx context.Context) error
	ResetVIA(ctx context.Context) error
	GetRoomCode(ctx context.Context) (string, error)
	IsConnected(ctx context.Context) bool
	GetHardwareInfo(ctx context.Context) (structs.HardwareInfo, error)
	GetStatusOfUsers(ctx context.Context) (structs.VIAUsers, error)
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

		d, err := create(c.Request().Context())
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
		log.L.Debugf("Value passed by SetViaVolume is %v", value)

		volume, err := strconv.Atoi(value)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err.Error())
		} else if volume > 100 || volume < 1 {
			log.L.Debugf("Volume command error - volume value %s is outside the bounds of 1-100", value)
			return c.JSON(http.StatusBadRequest, "Error: volume must be a value from 1 to 100!")
		}

		//volumec := strconv.Itoa(value)
		log.L.Debugf("Setting volume for %s to %v...", address, volume)

		d, err := create(c.Request().Context(), address)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		response, err := d.SetVolume(c.Request().Context(), value)

		if err != nil {
			log.L.Debugf("An Error Occured: %s", err)
			return c.JSON(http.StatusBadRequest, "An error has occured while setting volume")
		}
		log.L.Debugf("Success: %s", response)

		return c.JSON(http.StatusOK, status.Volume{Volume: volume})

	})
	// VIA Reset and Rebooting of Endpoints
	e.GET("/:address/reset", func(c echo.Context) error {

		address := c.Param("address")

		d, err := create(c.Request().Context(), address)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		err = d.ResetVIA(c.Request().Context())
		if err != nil {
			log.L.Debugf("There was a problem: %v", err.Error())
			return c.JSON(http.StatusInternalServerError, err.Error())
		}

		log.L.Debugf("Success.")

		return c.JSON(http.StatusOK, "Success")
	})

	e.GET("/:address/reboot", func(c echo.Context) error {
		address := c.Param("address")

		d, err := create(c.Request().Context(), address)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		err = d.RebootVIA(c.Request().Context())
		if err != nil {
			log.L.Debugf("There was a problem: %v", err.Error())
			return c.JSON(http.StatusInternalServerError, err.Error())
		}

		log.L.Debugf("Success.")

		return c.JSON(http.StatusOK, "Success")
	})

	// Informational Endpoints
	e.GET("/:address/connected", func(c echo.Context) error {
		address := c.Param("address")

		d, err := create(c.Request().Context(), address)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		connected := d.IsConnected(c.Request().Context(), address)

		if connected {
			log.L.Debugf("%s is connected", address)
		} else {
			log.L.Debugf("%s is not connected", address)
		}

		return c.JSON(http.StatusOK, connected)

	})

	e.GET("/:address/hardware", func(c echo.Context) error {
		address := c.Param("address")

		d, err := create(c.Request().Context(), address)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		hardware, err := d.GetHardwareInfo(c.Request().Context(), address)
		if err != nil {
			log.L.Debugf("Error getting hardware status: %s", err.Error())
			return c.JSON(http.StatusInternalServerError, err.Error())
		}

		return c.JSON(http.StatusOK, hardware)
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
	e.GET("/:address/roomcode", func(c echo.Context) error {
		address := c.Param("address")

		d, err := create(c.Request().Context(), address)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		code, err := d.GetRoomCode(c.Request().Context())
		if err != nil {
			log.L.Errorf("Failed to retrieve VIA room code: %s", err.Error())
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
			log.L.Errorf("Failed to retrieve current user list: %s", err.Error())
			return c.JSON(http.StatusInternalServerError, err)
		}

		return c.JSON(http.StatusOK, userlist)
	})
}
