module github.com/byuoitav/via-control

go 1.13

replace github.com/byuoitav/kramer-driver => /home/creeder/go/src/github.com/byuoitav/kramer-driver

require (
	github.com/byuoitav/av-control-api v0.3.2
	github.com/byuoitav/central-event-system v0.0.0-20200121172633-64fd9d467249
	github.com/byuoitav/common v0.0.0-20191210190714-e9b411b3cc0d
	github.com/byuoitav/kramer-driver v0.0.0-20200109164211-27eaf3a97894
	github.com/byuoitav/kramer-microservice v0.0.0-20190827223429-01781d8ea02e
	github.com/fatih/color v1.9.0
	github.com/labstack/echo v3.3.10+incompatible
	github.com/spf13/pflag v1.0.5
	go.uber.org/zap v1.14.0 // indirect
)
