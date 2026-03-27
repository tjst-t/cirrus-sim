module github.com/tjst-t/cirrus-sim/cmd/cirrus-sim

go 1.24.0

require (
	github.com/tjst-t/cirrus-sim/awx-sim v0.0.0
	github.com/tjst-t/cirrus-sim/common v0.0.0
	github.com/tjst-t/cirrus-sim/libvirt-sim v0.0.0
	github.com/tjst-t/cirrus-sim/netbox-sim v0.0.0
	github.com/tjst-t/cirrus-sim/ovn-sim v0.0.0
	github.com/tjst-t/cirrus-sim/postgres v0.0.0
	github.com/tjst-t/cirrus-sim/storage-sim v0.0.0
	github.com/tjst-t/cirrus-sim/webui v0.0.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/fergusstrange/embedded-postgres v1.34.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
)

replace (
	github.com/tjst-t/cirrus-sim/awx-sim => ../../awx-sim
	github.com/tjst-t/cirrus-sim/common => ../../common
	github.com/tjst-t/cirrus-sim/libvirt-sim => ../../libvirt-sim
	github.com/tjst-t/cirrus-sim/netbox-sim => ../../netbox-sim
	github.com/tjst-t/cirrus-sim/ovn-sim => ../../ovn-sim
	github.com/tjst-t/cirrus-sim/postgres => ../../postgres
	github.com/tjst-t/cirrus-sim/storage-sim => ../../storage-sim
	github.com/tjst-t/cirrus-sim/webui => ../../webui
)
