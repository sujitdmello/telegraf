package azure_iothub

// azure_iothub.go

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	iothub "github.com/amenzhinsky/iothub/iotdevice"
	iotmqtt "github.com/amenzhinsky/iothub/iotdevice/transport/mqtt"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/outputs"
	"github.com/influxdata/telegraf/plugins/serializers"
)

// Iothub struct
type Iothub struct {
	Client              *iothub.ModuleClient
	UseGateway          bool   `toml:"use_gateway"`
	ConnectionString    string `toml:"connection_string"`
	HubName             string `toml:"hub_name"`
	DeviceID            string `toml:"device_id"`
	ModuleID            string `toml:"module_id"`
	SharedAccessKey     string `toml:"shared_access_key"`
	SharedAccessKeyName string `toml:"shared_access_key_name"`
	serializer          serializers.Serializer
}

// Description returns plugin description
func (i *Iothub) Description() string {
	return "Output plugin for Azure IoT Hub Edge Module"
}

// SampleConfig returns a sample configuration
func (i *Iothub) SampleConfig() string {
	return `
	## One of the following sets required for configuration:
	#  
	## For use on IoT Edge:
	#
	# use_gateway = true
	#
	## To specify a device/module connection string:
	#
	#  # 1.
	#  connection_string = ""
	#  use_gateway = true
	#
	## To use a shared access key to form a connection string
	#
	#  # 2.
	#  hub_name = ""
	#  device_id = ""
	#  module_id = ""
	#  shared_access_key = ""
	#  use_gateway = true
`
}

func (i *Iothub) hasConnectionString() bool {

	if len(strings.TrimSpace(i.ConnectionString)) > 0 {
		return true
	}

	return false
}

func (i *Iothub) hasHubName() bool {

	if len(strings.TrimSpace(i.HubName)) > 0 {
		return true
	}

	return false
}

func (i *Iothub) hasSharedAccessKey() bool {

	if len(strings.TrimSpace(i.SharedAccessKey)) > 0 {
		return true
	}

	return false
}

func (i *Iothub) hasSharedAccessKeyName() bool {

	if len(strings.TrimSpace(i.SharedAccessKeyName)) > 0 {
		return true
	}

	return false
}

func (i *Iothub) hasDeviceID() bool {

	if len(strings.TrimSpace(i.DeviceID)) > 0 {
		return true
	}

	return false
}

func (i *Iothub) hasModuleID() bool {

	if len(strings.TrimSpace(i.ModuleID)) > 0 {
		return true
	}

	return false
}

func (i *Iothub) createConnectionString() {
	conn := fmt.Sprintf("HostName=%s", i.HubName)

	if i.hasDeviceID() {
		conn = fmt.Sprintf("%s;DeviceId=%s", conn, i.DeviceID)
	}

	if i.hasModuleID() {
		conn = fmt.Sprintf("%s;ModuleId=%s", conn, i.ModuleID)
	}

	if i.hasSharedAccessKeyName() {
		conn = fmt.Sprintf("%s;SharedAccessKeyName=%s", conn, i.SharedAccessKeyName)
	}

	if i.hasSharedAccessKey() {
		conn = fmt.Sprintf("%s;SharedAccessKey=%s", conn, i.SharedAccessKey)
	}

	i.ConnectionString = conn
}

func (i *Iothub) validateConfiguration() bool {
	valid := false

	// connection_string provided
	if i.hasConnectionString() {
		valid = true
	}

	// hub_name, shared_access_key, and shared_access_key_name provided
	if i.hasHubName() && i.hasSharedAccessKey() && i.hasSharedAccessKeyName() {
		valid = true
	}

	// hub_name, shared_access_key, and device_id provided
	if i.hasHubName() && i.hasSharedAccessKey() && i.hasDeviceID() {
		valid = true
	}

	return valid
}

// Init IoT Hub
func (i *Iothub) Init() error {

	// check for a valid configuration
	valid := i.validateConfiguration()

	if valid {

		// if there's no explict connection string given
		if !i.hasConnectionString() {
			// create connection string from IoT Hub configuration
			i.createConnectionString()
		}

		// create a new client from connection string
		gwhn := os.Getenv("IOTEDGE_GATEWAYHOSTNAME")
		mgid := os.Getenv("IOTEDGE_MODULEGENERATIONID")
		wluri := os.Getenv("IOTEDGE_WORKLOADURI")

		c, err := iothub.NewModuleFromConnectionString(
			iotmqtt.NewModuleTransport(), i.ConnectionString, gwhn, mgid, wluri, true,
		)
		if err != nil {
			return err
		}

		// set IoT Hub client
		i.Client = c

		s, err := serializers.NewJsonSerializer(time.Second)
		if err != nil {
			return err
		}
		i.serializer = s

		return err
	} else {

		// New client from Environment variables. Applicable for deployment to IoT Edge
		c, err := iothub.NewModuleFromEnvironment(
			iotmqtt.NewModuleTransport(), true,
		)
		if err != nil {
			return err
		}

		// set IoT Hub client
		i.Client = c

		s, err := serializers.NewJsonSerializer(time.Second)
		if err != nil {
			return err
		}
		i.serializer = s

		return err
	}
}

var connected = false

// Connect IoT Hub Client
func (i *Iothub) Connect() error {
	if !connected {
		err := i.Client.Connect(context.Background())
		if err != nil {
			connected = true
		}
		return err
	}
	return nil
}

// Close IoT Hub Client connection
func (i *Iothub) Close() error {
	err := i.Client.Close()
	return err
}

// Write Telegraf metrics to IoT Hub
func (i *Iothub) Write(metrics []telegraf.Metric) error {

	b, err := i.serializer.SerializeBatch(metrics)
	if err == nil {
		err = i.Client.SendEvent(context.Background(), b)
	}
	return err
}

func init() {
	outputs.Add("azure_iothub", func() telegraf.Output { return &Iothub{} })
}
