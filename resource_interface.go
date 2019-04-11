package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"path"

	"github.com/aristanetworks/goarista/gnmi"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/networkop/tf-yang/pkg/ocintf"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
)

var lookupPath = map[string]string{
	"description": "/interfaces/interface[name=%s]/config/description",
	"vlan":        "/interfaces/interface[name=%s]/ethernet/switched-vlan/config",
	"ipv4":        "/interfaces/interface[name=%s]/subinterfaces/subinterface[index=0]/ipv4",
	"global":      "/interfaces/interface[name=%s]",
}

func resourceInterface() *schema.Resource {
	return &schema.Resource{
		Create: resourceInterfaceCreate,
		Read:   resourceInterfaceRead,
		Update: resourceInterfaceUpdate,
		Delete: resourceInterfaceDelete,

		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"description": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"switchport": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
				ForceNew: true,
			},
			"access_vlan": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
			},
			"trunk_vlans": &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeInt},
			},
			"ipv4_address": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
}

func resourceInterfaceCreate(d *schema.ResourceData, meta interface{}) error {
	c := *meta.(*MyClient)
	oc := &ocintf.OpenconfigInterfaces_Interfaces{}

	name := d.Get("name").(string)

	newIntf, _ := oc.NewInterface(name)
	ygot.BuildEmptyTree(newIntf)

	subIntf, _ := newIntf.Subinterfaces.NewSubinterface(0)
	ygot.BuildEmptyTree(subIntf)

	if v, ok := d.GetOk("description"); ok {
		newIntf.Config.Description = ygot.String(v.(string))
	}

	switchport := d.Get("switchport").(bool)

	if !switchport {
		log.Printf("[DEBUG] [GNMI] Routed Interface Case")
		subIntf.Ipv4.Config.Enabled = ygot.Bool(true)

		if v, ok := d.GetOk("ipv4_address"); ok {
			if err := assignIPv4Address(subIntf, v.(string)); err != nil {
				return fmt.Errorf("Failed to create IPv4 address: %+v", err)
			}

		}
	} else {
		log.Printf("[DEBUG] [GNMI] Switchport case")
		subIntf.Ipv4.Config.Enabled = ygot.Bool(false)

		if v, ok := d.GetOk("access_vlan"); ok {
			newIntf.Ethernet.SwitchedVlan.Config.AccessVlan = ygot.Uint16(uint16(v.(int)))
			newIntf.Ethernet.SwitchedVlan.Config.AristaInterfaceMode = ygot.String("ACCESS")
		}
		if v, ok := d.GetOk("trunk_vlans"); ok {
			for _, vlan := range v.([]interface{}) {
				trunkVlan := &ocintf.OpenconfigInterfaces_Interfaces_Interface_Ethernet_SwitchedVlan_Config_TrunkVlans_Union_Uint16{
					Uint16: uint16(vlan.(int))}
				newIntf.Ethernet.SwitchedVlan.Config.TrunkVlans = append(newIntf.Ethernet.SwitchedVlan.Config.TrunkVlans, trunkVlan)
				newIntf.Ethernet.SwitchedVlan.Config.AristaInterfaceMode = ygot.String("TRUNK")
			}
		}
	}

	key := fmt.Sprintf(lookupPath["global"], name)
	if err := createPath(c.Context, *c.Client, key, newIntf); err != nil {
		return fmt.Errorf("Failed to create resource: %+v", err)
	}
	d.SetId(name)

	return resourceInterfaceRead(d, meta)
}

func resourceInterfaceRead(d *schema.ResourceData, meta interface{}) error {
	c := *meta.(*MyClient).Client
	ctx := meta.(*MyClient).Context
	readIntf := &ocintf.OpenconfigInterfaces_Interfaces_Interface{}

	gnmiPaths := gnmi.SplitPaths([]string{fmt.Sprintf(lookupPath["global"], d.Id())})
	req, err := gnmi.NewGetRequest(gnmiPaths, "")
	if err != nil {
		return err
	}

	resp, err := c.Get(ctx, req)
	if err != nil {
		return err
	}

	for _, notif := range resp.Notification {
		prefix := gnmi.StrPath(notif.Prefix)
		for _, update := range notif.Update {
			if err := ocintf.Unmarshal([]byte(gnmi.StrUpdateVal(update)), readIntf); err != nil {
				panic(fmt.Sprintf("Can't unmarshal JSON: %v", err))
			}
			log.Printf("[DEBUG] [GNMI] Read path: %s", path.Join(prefix, gnmi.StrPath(update.Path)))
			log.Printf("[DEBUG] [GNMI] JSON: %s", path.Join(gnmi.StrUpdateVal(update)))

			d.Set("description", readIntf.Config.Description)

			//d.Set("access_vlan", readIntf.Ethernet.SwitchedVlan.Config.AccessVlan)

			//d.Set("trunk_vlans", readIntf.Ethernet.SwitchedVlan.Config.TrunkVlans)

		}
	}

	return nil
}

func resourceInterfaceUpdate(d *schema.ResourceData, meta interface{}) error {
	c := *meta.(*MyClient)
	oc := &ocintf.OpenconfigInterfaces_Interfaces{}
	d.Partial(true)

	myIntf, _ := oc.NewInterface(d.Id())
	ygot.BuildEmptyTree(myIntf)
	myIntf.Config.Name = ygot.String(d.Id())

	if d.HasChange("description") {
		key := fmt.Sprintf(lookupPath["description"], d.Id())
		updatePath(c.Context, *c.Client, key, ygot.String(d.Get("description").(string)))

		d.SetPartial("description")
	}

	if d.HasChange("ipv4_address") {
		key := fmt.Sprintf(lookupPath["ipv4"], d.Id())
		subIntf, _ := myIntf.Subinterfaces.NewSubinterface(0)
		ygot.BuildEmptyTree(subIntf)

		if v, ok := d.GetOk("ipv4_address"); ok {
			if err := assignIPv4Address(subIntf, v.(string)); err != nil {
				return fmt.Errorf("Failed to create IPv4 address: %+v", err)
			}

		}

		// EOS doesn't do gnmi replace correctly and creates secondary IPs instead.
		// So I have to delete the old interface before the update
		if err := deletePath(c.Context, *c.Client, key); err != nil {
			return fmt.Errorf("Failed to cleanup old IPv4 address: %+v", err)
		}

		updatePath(c.Context, *c.Client, key, subIntf.Ipv4)

		d.SetPartial("ipv4_address")
	}

	if d.HasChange("access_vlan") {
		key := fmt.Sprintf(lookupPath["vlan"], d.Id())
		if v, ok := d.GetOk("access_vlan"); ok {
			myIntf.Ethernet.SwitchedVlan.Config.AccessVlan = ygot.Uint16(uint16(v.(int)))
			myIntf.Ethernet.SwitchedVlan.Config.AristaInterfaceMode = ygot.String("ACCESS")

			updatePath(c.Context, *c.Client, key, myIntf.Ethernet.SwitchedVlan.Config)
		} else {
			if err := deletePath(c.Context, *c.Client, key); err != nil {
				return fmt.Errorf("Failed to cleanup old IPv4 address: %+v", err)
			}
		}

		d.SetPartial("access_vlan")
	}

	if d.HasChange("trunk_vlans") {
		key := fmt.Sprintf(lookupPath["vlan"], d.Id())

		if v, ok := d.GetOk("trunk_vlans"); ok {
			for _, vlan := range v.([]interface{}) {
				trunkVlan := &ocintf.OpenconfigInterfaces_Interfaces_Interface_Ethernet_SwitchedVlan_Config_TrunkVlans_Union_Uint16{
					Uint16: uint16(vlan.(int))}
				myIntf.Ethernet.SwitchedVlan.Config.TrunkVlans = append(myIntf.Ethernet.SwitchedVlan.Config.TrunkVlans, trunkVlan)
				myIntf.Ethernet.SwitchedVlan.Config.AristaInterfaceMode = ygot.String("TRUNK")
			}

			updatePath(c.Context, *c.Client, key, myIntf.Ethernet.SwitchedVlan.Config)

			d.SetPartial("trunk_vlans")
		}
	}

	d.Partial(false)
	return resourceInterfaceRead(d, meta)
}

func resourceInterfaceDelete(d *schema.ResourceData, meta interface{}) error {
	c := *meta.(*MyClient)

	for el, path := range lookupPath {
		if el != "global" {
			if err := deletePath(c.Context, *c.Client, fmt.Sprintf(path, d.Id())); err != nil {
				return fmt.Errorf("Failed to delete %s path", fmt.Sprintf(path, d.Id()))
			}
		}
	}

	d.SetId("")
	return nil
}

func gnmiDo(ctx context.Context, c gpb.GNMIClient, path, opType string, yangElm interface{}) (err error) {
	var setOps []*gnmi.Operation
	var json string

	if v, ok := yangElm.(ygot.ValidatedGoStruct); ok {
		json, err = ygot.EmitJSON(v, &ygot.EmitJSONConfig{
			Format: ygot.RFC7951,
			Indent: "  ",
			RFC7951Config: &ygot.RFC7951JSONConfig{
				AppendModuleName: true,
			},
		})
		if err != nil {
			return fmt.Errorf("Failed to Create JSON Update: %+v", err)
		}
	} else if v, ok := yangElm.(*string); ok {
		json = *v
	} else if yangElm != nil {
		log.Printf("[DEBUG] [GNMI] Invalid ygot struct: %+v", yangElm)
	}

	op := &gnmi.Operation{
		Type: opType,
		Path: gnmi.SplitPath(path),
	}
	if opType != "delete" && json != "" {
		op.Val = json
	}
	setOps = append(setOps, op)

	log.Printf("[DEBUG] [GNMI] %s path %s with: %+v", opType, path, json)
	if err := gnmi.Set(ctx, c, setOps); err != nil {
		return err
	}

	return nil
}

func deletePath(ctx context.Context, c gpb.GNMIClient, p string) error {
	return gnmiDo(ctx, c, p, "delete", nil)
}

func updatePath(ctx context.Context, c gpb.GNMIClient, p string, yangElm interface{}) error {
	return gnmiDo(ctx, c, p, "replace", yangElm)
}

func createPath(ctx context.Context, c gpb.GNMIClient, p string, yangElm interface{}) error {
	return gnmiDo(ctx, c, p, "update", yangElm)
}

func assignIPv4Address(subIntf *ocintf.OpenconfigInterfaces_Interfaces_Interface_Subinterfaces_Subinterface, ipv4 string) error {
	ipv4Addr, ipv4Net, err := net.ParseCIDR(ipv4)
	if err != nil {
		return fmt.Errorf("Failed to parse IPv4 address: %+v", err)
	}

	a, _ := subIntf.Ipv4.Addresses.NewAddress(ipv4Addr.String())
	ygot.BuildEmptyTree(a)

	prefixLen, _ := ipv4Net.Mask.Size()
	a.Config.Ip = ygot.String(ipv4Addr.String())
	a.Config.PrefixLength = ygot.Uint8(uint8(prefixLen))

	return nil
}
