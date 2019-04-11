# tf-yang
A custom Terraform provider for gNMI implementing the following resources and their attributes:

* Interface
  * Description
  * switchport
  * IPv4 Address
  * Access VLAN
  * Trunk VLANs

## Build

```
go build -o terraform-provider-gnmi
```

## Use

Standard TF API (e.g. plan,apply,delete) can be used to manipulate network resources. See `main.tf` and `provider.tf` for an example.