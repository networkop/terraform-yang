provider "gnmi" {
  alias    = "s1"
  address = "localhost:6030"
  username   = "admin"
  password   = "admin"
}

provider "gnmi" {
  alias    = "s2"
  address = "localhost:6031"
  username   = "admin"
  password   = "admin"
}