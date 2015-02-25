package dymoscale

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/dcarley/gousb/usb"
)

const VendorID usb.ID = 0x0922 // Dymo, all devices

// Measurement represents a parsed reading from the scale.
type Measurement struct {
	AlwaysThree int8  // Don't know what this is but it's always 3
	Stability   int8  // How accurate the measurement was
	Mode        int8  // Grams or Ounces
	ScaleFactor int8  // WeightMinor*10^n when Mode is Ounces
	WeightMinor uint8 //
	WeightMajor uint8 // Overflow for WeightMinor, n*256
}

type Scale struct {
	context  *usb.Context
	device   *usb.Device
	endpoint usb.Endpoint
}

// closeWithError closes all outstanding devices and the context, then
// returns the original error.
func closeWithError(context *usb.Context, devices []*usb.Device, err error) (*Scale, error) {
	for _, dev := range devices {
		dev.Close()
	}
	context.Close()

	return nil, err
}

// ReadMeasurement obtains a Measurement from an io.Reader.
func ReadMeasurement(reader io.Reader) (Measurement, error) {
	var reading Measurement
	err := binary.Read(reader, binary.LittleEndian, &reading)

	return reading, err
}

// NewScale opens a connection to a Dymo USB scale. You MUST call Close()
// when you're finished.
func NewScale() (*Scale, error) {
	ctx := usb.NewContext()

	devs, err := ctx.ListDevices(func(desc *usb.Descriptor) bool {
		if desc.Vendor == VendorID {
			return true
		}

		return false
	})
	if err != nil {
		return closeWithError(ctx, devs, err)
	}

	if len(devs) != 1 {
		err := fmt.Errorf("expected 1 device, found %d", len(devs))
		return closeWithError(ctx, devs, err)
	}

	dev := devs[0]
	ep, err := dev.OpenEndpoint(
		dev.Configs[0].Config,
		dev.Configs[0].Interfaces[0].Number,
		dev.Configs[0].Interfaces[0].Setups[0].Number,
		dev.Configs[0].Interfaces[0].Setups[0].Endpoints[0].Address,
	)
	if err != nil {
		return closeWithError(ctx, devs, err)
	}

	scale := &Scale{
		context:  ctx,
		device:   dev,
		endpoint: ep,
	}

	return scale, nil
}

// ReadRaw gets a raw reading from the scale.
func (s *Scale) ReadRaw() ([]byte, error) {
	buf := make([]byte, s.endpoint.Info().MaxPacketSize)
	_, err := s.endpoint.Read(buf)

	return buf, err
}

// ReadMeasurement returns a parsed Measurement from the scale.
func (s *Scale) ReadMeasurement() (Measurement, error) {
	// TODO: Reset on libusb errors here?
	return ReadMeasurement(s.endpoint)
}

// Close closes the USB device and context. If there are any errors then the
// inner-most is returned, but both will still attempt to be closed.
func (s *Scale) Close() error {
	errDev := s.device.Close()
	errCtx := s.context.Close()

	if errDev != nil {
		return errDev
	}
	return errCtx
}
