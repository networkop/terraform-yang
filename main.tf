resource "gnmi_interface" "SW1_Eth1" {
    provider = "gnmi.s1"
    name = "Ethernet1"
    description = "TF_INT_ETH1"
    switchport = false
    ipv4_address = "12.12.12.1/24"
}
resource "gnmi_interface" "SW2_Eth1" {
    provider = "gnmi.s2"
    name = "Ethernet1"
    description = "TF_INT_ETH1"
    switchport = false
    ipv4_address = "12.12.12.2/24"
}

#resource "gnmi_interface" "Ethernet49" {
#    name = "Ethernet49"
#    description = "TF_INT_ETH1_3"
#    switchport = true
#    trunk_vlans = [100, 201]
#    #access_vlan = 180
#}