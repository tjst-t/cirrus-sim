package ovsdb

// ColumnSchema defines the schema for a single column.
type ColumnSchema struct {
	Type      string // "string", "integer", "boolean", "real", "uuid", "set", "map"
	KeyType   string // for set/map: element type
	ValueType string // for map: value type
	RefTable  string // for uuid refs: target table name
	Optional  bool   // whether the column can be absent
}

// TableSchema defines the schema for a table.
type TableSchema struct {
	Columns map[string]ColumnSchema
}

// OVNNBTables is the simplified OVN Northbound schema.
var OVNNBTables = map[string]TableSchema{
	"Logical_Switch": {
		Columns: map[string]ColumnSchema{
			"name":         {Type: "string"},
			"ports":        {Type: "set", KeyType: "uuid", RefTable: "Logical_Switch_Port"},
			"acls":         {Type: "set", KeyType: "uuid", RefTable: "ACL"},
			"dns_records":  {Type: "set", KeyType: "uuid", RefTable: "DNS"},
			"other_config": {Type: "map", KeyType: "string", ValueType: "string"},
			"external_ids": {Type: "map", KeyType: "string", ValueType: "string"},
		},
	},
	"Logical_Switch_Port": {
		Columns: map[string]ColumnSchema{
			"name":           {Type: "string"},
			"type":           {Type: "string"},
			"addresses":      {Type: "set", KeyType: "string"},
			"port_security":  {Type: "set", KeyType: "string"},
			"up":             {Type: "boolean", Optional: true},
			"enabled":        {Type: "boolean", Optional: true},
			"dhcpv4_options": {Type: "uuid", RefTable: "DHCP_Options", Optional: true},
			"options":        {Type: "map", KeyType: "string", ValueType: "string"},
			"external_ids":   {Type: "map", KeyType: "string", ValueType: "string"},
			"tag":            {Type: "integer", Optional: true},
		},
	},
	"Logical_Router": {
		Columns: map[string]ColumnSchema{
			"name":          {Type: "string"},
			"ports":         {Type: "set", KeyType: "uuid", RefTable: "Logical_Router_Port"},
			"static_routes": {Type: "set", KeyType: "uuid", RefTable: "Logical_Router_Static_Route"},
			"nat":           {Type: "set", KeyType: "uuid", RefTable: "NAT"},
			"enabled":       {Type: "boolean", Optional: true},
			"options":       {Type: "map", KeyType: "string", ValueType: "string"},
			"external_ids":  {Type: "map", KeyType: "string", ValueType: "string"},
		},
	},
	"Logical_Router_Port": {
		Columns: map[string]ColumnSchema{
			"name":         {Type: "string"},
			"mac":          {Type: "string"},
			"networks":     {Type: "set", KeyType: "string"},
			"options":      {Type: "map", KeyType: "string", ValueType: "string"},
			"external_ids": {Type: "map", KeyType: "string", ValueType: "string"},
		},
	},
	"Logical_Router_Static_Route": {
		Columns: map[string]ColumnSchema{
			"ip_prefix":    {Type: "string"},
			"nexthop":      {Type: "string"},
			"output_port":  {Type: "string", Optional: true},
			"external_ids": {Type: "map", KeyType: "string", ValueType: "string"},
		},
	},
	"ACL": {
		Columns: map[string]ColumnSchema{
			"action":       {Type: "string"},
			"direction":    {Type: "string"},
			"match":        {Type: "string"},
			"priority":     {Type: "integer"},
			"external_ids": {Type: "map", KeyType: "string", ValueType: "string"},
		},
	},
	"NAT": {
		Columns: map[string]ColumnSchema{
			"type":         {Type: "string"},
			"external_ip":  {Type: "string"},
			"logical_ip":   {Type: "string"},
			"logical_port": {Type: "string", Optional: true},
			"external_ids": {Type: "map", KeyType: "string", ValueType: "string"},
		},
	},
	"DHCP_Options": {
		Columns: map[string]ColumnSchema{
			"cidr":         {Type: "string"},
			"options":      {Type: "map", KeyType: "string", ValueType: "string"},
			"external_ids": {Type: "map", KeyType: "string", ValueType: "string"},
		},
	},
	"DNS": {
		Columns: map[string]ColumnSchema{
			"records":      {Type: "map", KeyType: "string", ValueType: "string"},
			"external_ids": {Type: "map", KeyType: "string", ValueType: "string"},
		},
	},
	"Load_Balancer": {
		Columns: map[string]ColumnSchema{
			"name":         {Type: "string"},
			"vips":         {Type: "map", KeyType: "string", ValueType: "string"},
			"protocol":     {Type: "string", Optional: true},
			"external_ids": {Type: "map", KeyType: "string", ValueType: "string"},
		},
	},
	"Address_Set": {
		Columns: map[string]ColumnSchema{
			"name":         {Type: "string"},
			"addresses":    {Type: "set", KeyType: "string"},
			"external_ids": {Type: "map", KeyType: "string", ValueType: "string"},
		},
	},
	"Port_Group": {
		Columns: map[string]ColumnSchema{
			"name":         {Type: "string"},
			"ports":        {Type: "set", KeyType: "uuid"},
			"acls":         {Type: "set", KeyType: "uuid", RefTable: "ACL"},
			"external_ids": {Type: "map", KeyType: "string", ValueType: "string"},
		},
	},
	"Gateway_Chassis": {
		Columns: map[string]ColumnSchema{
			"name":          {Type: "string"},
			"chassis_name":  {Type: "string"},
			"priority":      {Type: "integer"},
			"external_ids":  {Type: "map", KeyType: "string", ValueType: "string"},
		},
	},
}

// SchemaJSON returns a simplified OVN NB schema as a map suitable for JSON serialization.
func SchemaJSON() map[string]interface{} {
	tables := make(map[string]interface{})
	for tName, tSchema := range OVNNBTables {
		cols := make(map[string]interface{})
		for cName, cSchema := range tSchema.Columns {
			col := buildColumnType(cSchema)
			cols[cName] = map[string]interface{}{"type": col}
		}
		// Add implicit _uuid column
		tables[tName] = map[string]interface{}{
			"columns": cols,
		}
	}
	return map[string]interface{}{
		"name":    "OVN_Northbound",
		"version": "7.3.0",
		"tables":  tables,
	}
}

// OVNICTables is the simplified OVN IC Northbound schema for inter-cluster connectivity.
var OVNICTables = map[string]TableSchema{
	"Transit_Switch": {
		Columns: map[string]ColumnSchema{
			"name":         {Type: "string"},
			"external_ids": {Type: "map", KeyType: "string", ValueType: "string"},
		},
	},
	"Availability_Zone": {
		Columns: map[string]ColumnSchema{
			"name":         {Type: "string"},
			"external_ids": {Type: "map", KeyType: "string", ValueType: "string"},
		},
	},
	"Route": {
		Columns: map[string]ColumnSchema{
			"ip_prefix":         {Type: "string"},
			"nexthop":           {Type: "string"},
			"availability_zone": {Type: "uuid", RefTable: "Availability_Zone", Optional: true},
			"transit_switch":    {Type: "uuid", RefTable: "Transit_Switch", Optional: true},
			"external_ids":      {Type: "map", KeyType: "string", ValueType: "string"},
		},
	},
	"Port_Binding": {
		Columns: map[string]ColumnSchema{
			"logical_port":      {Type: "string"},
			"transit_switch":    {Type: "uuid", RefTable: "Transit_Switch", Optional: true},
			"availability_zone": {Type: "uuid", RefTable: "Availability_Zone", Optional: true},
			"address":           {Type: "set", KeyType: "string"},
			"external_ids":      {Type: "map", KeyType: "string", ValueType: "string"},
		},
	},
}

// ICSchemaJSON returns the OVN IC Northbound schema as a JSON-serializable object.
func ICSchemaJSON() interface{} {
	tables := make(map[string]interface{})
	for tName, tSchema := range OVNICTables {
		cols := make(map[string]interface{})
		for cName, cSchema := range tSchema.Columns {
			cols[cName] = map[string]interface{}{"type": buildColumnType(cSchema)}
		}
		tables[tName] = map[string]interface{}{"columns": cols}
	}
	return map[string]interface{}{
		"name":    "OVN_IC_Northbound",
		"version": "1.0.0",
		"tables":  tables,
	}
}

func buildColumnType(cs ColumnSchema) interface{} {
	switch cs.Type {
	case "string":
		if cs.Optional {
			return map[string]interface{}{
				"key": "string", "min": 0, "max": 1,
			}
		}
		return "string"
	case "integer":
		if cs.Optional {
			return map[string]interface{}{
				"key": "integer", "min": 0, "max": 1,
			}
		}
		return "integer"
	case "boolean":
		if cs.Optional {
			return map[string]interface{}{
				"key": "boolean", "min": 0, "max": 1,
			}
		}
		return "boolean"
	case "real":
		if cs.Optional {
			return map[string]interface{}{
				"key": "real", "min": 0, "max": 1,
			}
		}
		return "real"
	case "uuid":
		key := map[string]interface{}{"type": "uuid"}
		if cs.RefTable != "" {
			key["refTable"] = cs.RefTable
		}
		if cs.Optional {
			return map[string]interface{}{"key": key, "min": 0, "max": 1}
		}
		return map[string]interface{}{"key": key}
	case "set":
		key := map[string]interface{}{"type": cs.KeyType}
		if cs.RefTable != "" {
			key["refTable"] = cs.RefTable
		}
		return map[string]interface{}{
			"key": key, "min": 0, "max": "unlimited",
		}
	case "map":
		return map[string]interface{}{
			"key":   cs.KeyType,
			"value": cs.ValueType,
			"min":   0,
			"max":   "unlimited",
		}
	default:
		return cs.Type
	}
}
