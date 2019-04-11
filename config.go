package main

import (
	"context"
	"fmt"

	"github.com/aristanetworks/goarista/gnmi"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

// MyClient contains the handle to the device's gnmi client
type MyClient struct {
	Client  *gpb.GNMIClient
	Context context.Context
}

func getGNMIClient(cfg *gnmi.Config) (*MyClient, error) {
	ctx := gnmi.NewContext(context.Background(), cfg)

	client, err := gnmi.Dial(cfg)
	if err != nil {
		return nil, fmt.Errorf("Could not connect to device: %+v", err)
	}

	return &MyClient{&client, ctx}, nil
}
